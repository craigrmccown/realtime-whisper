package predict

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

const (
	sep = "<|SEP|>"
)

func TestEmitter(t *testing.T) {
	b, err := os.ReadFile("testdata/sample.txt")
	if err != nil {
		log.Fatal(err)
	}
	windows := strings.Split(string(b), sep)

	windowCh := make(chan string, len(windows))
	for _, window := range windows {
		windowCh <- window
	}
	close(windowCh)

	tokenCh := make(chan string, 1024)
	emitter := NewEmitter(5, 3)
	if err := emitter.Do(windowCh, tokenCh); err != nil {
		log.Fatal(err)
	}

	var tokens []string
	for tok := range tokenCh {
		tokens = append(tokens, tok)
	}

	fmt.Println(tokens)
}
