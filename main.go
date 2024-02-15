package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path"
	"time"

	"github.com/craigrmccown/windowed-markov/predict"
	"github.com/craigrmccown/windowed-markov/utility"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
	"golang.org/x/sync/errgroup"
)

const (
	// Whisper requires 16kHz audio sampling
	sampleRate = 16000
)

var (
	fWhisperUrl     = flag.String("whisper-url", "http://localhost:8080/inference", "the URL used to address the Whisper server")
	fRecordFor      = flag.Duration("record-for", time.Second*30, "will record for the specified duration, then stop")
	fWindowDepth    = flag.Int("W", 3, "the number of windows to store in the window buffer")
	fWindowDuration = flag.Duration("D", time.Second*4, "the fixed duration of each window")
	fWindowStep     = flag.Duration("S", time.Second/2, "the amount of time to wait before producing the next window")
	fTokenLookback  = flag.Int("N", 3, "the n-gram length used for token prediction")
)

func main() {
	tempDir, err := os.MkdirTemp("/tmp", "whisper-audio-*")
	if err != nil {
		log.Fatalf("failed to create temporary directory for audio files")
	}
	defer os.RemoveAll(tempDir)

	portaudio.Initialize()
	defer portaudio.Terminate()

	// Buffer up to one full window of raw audio in memory, which should be
	// plenty since we are immediately encoding as wav and flushing to disk
	audioCh := make(chan []float32, int(sampleRate*(*fWindowDuration).Seconds()))
	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, sampleRate/3, func(chunk []float32) {
		audioCh <- chunk
	})
	if err != nil {
		log.Fatalf("failed to create microphone stream: %s", err)
	}

	if err := stream.Start(); err != nil {
		log.Fatalf("failed to open microphone stream: %s", err)
	}
	log.Println("microphone is live, listening...")

	g, ctx := errgroup.WithContext(context.Background())

	// Wait for interrupt or record-for to expire, then close microphone stream
	g.Go(func() error {
		defer close(audioCh)
		defer stream.Close()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		select {
		case <-time.After(*fRecordFor):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-sigCh:
			return fmt.Errorf("received interrupt signal: %s", sig)
		}
	})

	// Chop raw audio into sliding windows, encode them as wav, and write them
	// to disk. Then, transcribe each file and process transcriptions to predict
	// the speaker's words.
	windowCh := make(chan []float32, 1024)
	pathCh := make(chan string, 1024)
	textCh := make(chan string, 1024)
	tokenCh := make(chan string, 1024*16)

	g.Go(func() error {
		w := utility.NewWindower[float32](
			int(sampleRate*(*fWindowDuration).Seconds()),
			int(sampleRate*(*fWindowStep).Seconds()),
		)
		w.Do(audioCh, windowCh)
		return nil
	})

	g.Go(func() error {
		w := &WavWriter{baseDir: tempDir}
		return w.Do(windowCh, pathCh)
	})

	g.Go(func() error {
		t := &Transcriber{
			cli:      http.DefaultClient,
			endpoint: *fWhisperUrl,
		}
		return t.Do(pathCh, textCh)
	})

	g.Go(func() error {
		emitter := predict.NewEmitter(*fWindowDepth, *fTokenLookback)
		return emitter.Do(textCh, tokenCh)
	})

	g.Go(func() error {
		for token := range tokenCh {
			fmt.Printf("%s ", token)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

type Transcriber struct {
	cli      *http.Client
	endpoint string
	timeout  time.Duration
}

func (t *Transcriber) Do(inCh <-chan string, outCh chan<- string) error {
	defer close(outCh)

	for path := range inCh {
		text, err := t.transcribe(path)

		// If the timeout option is specified, skip windows when Whisper server
		// too long to respond
		if errors.Is(err, context.DeadlineExceeded) {
			continue
		} else if err != nil {
			return err
		}

		outCh <- text
	}

	return nil
}

func (t *Transcriber) transcribe(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	ctx := context.Background()
	if t.timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), *fWindowStep)
		defer cancel()
	}

	rsp, err := utility.DoMultipartUpload(ctx, t.cli, t.endpoint, f.Name(), f)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %w", err)
	}

	var j map[string]interface{}
	if err := json.Unmarshal(rsp, &j); err != nil {
		return "", fmt.Errorf("malformed response, could not parse JSON: %w", err)
	}

	if val, ok := j["text"]; !ok {
		return "", fmt.Errorf("malformed response, expected key 'text': %v", j)
	} else if text, ok := val.(string); !ok {
		return "", fmt.Errorf("malformed response, string 'text' expected: %v", j)
	} else {
		return text, nil
	}
}

type WavWriter struct {
	baseDir string
}

func (w *WavWriter) Do(inCh <-chan []float32, outCh chan<- string) error {
	defer close(outCh)

	for raw := range inCh {
		fPath := path.Join(w.baseDir, fmt.Sprintf("%d.wav", time.Now().UnixMilli()))

		if err := writeWav(raw, fPath); err != nil {
			return fmt.Errorf("failed to process window: %w", err)
		}

		outCh <- fPath
	}

	return nil
}

func writeWav(raw []float32, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  sampleRate,
		},
	}
	for _, f32Sample := range raw {
		// Convert raw floating point samples to 16 bit integer values and write
		// to audio buffer for encoding
		iSample := f32Sample * math.MaxInt16
		buf.Data = append(buf.Data, int(iSample))
	}

	e := wav.NewEncoder(f, sampleRate, 16, 1, 1)
	defer e.Close()

	if err := e.Write(&buf); err != nil {
		return err
	}

	return nil
}
