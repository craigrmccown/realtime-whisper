package predict

import (
	"math"
	"regexp"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/mb-14/gomarkov"
)

const (
	// We aim to emit tokens appearing in approximately the first 70% of the
	// current window
	progressTarget = 0.7

	// Used to compute edit distance. When two slices of tokens are being
	// compared at on offset where they don't fully overlap, this value is used
	// for tokens that aren't overlapping with the other slice.
	uncertaintyPenalty = 5
)

var (
	spacesPattern          = regexp.MustCompile(`\s+`)
	nonAlphanumericPattern = regexp.MustCompile(`[^A-Za-z0-9\s]+`)
	withinBracketsPattern  = regexp.MustCompile(`\[.*\]`)
)

type Emitter struct {
	depth, ngram    int
	q               *WindowQ
	emitted         []string
	tokensPerWindow float32
}

func NewEmitter(depth, ngram int) *Emitter {
	return &Emitter{
		depth:   depth,
		ngram:   ngram,
		q:       NewWindowQ(depth, ngram),
		emitted: make([]string, 0, ngram),
	}
}

func (e *Emitter) Do(inCh <-chan string, outCh chan<- string) error {
	defer close(outCh)

	for window := range inCh {
		window = sanitize(window)
		if window == "" {
			continue
		}

		tokens := tokenize(window)
		e.q.PushWindow(tokens)

		// Compute a weighted average to estimate the number of tokens per window
		fDepth := float32(e.depth)
		e.tokensPerWindow = (e.tokensPerWindow*(fDepth-1) + float32(len(tokens))) / fDepth

		if e.q.q.Full() {
			// Compute a progress, which is an estimate of how the emitted
			// tokens align with the current window
			idx := fuzzySearch(e.emitted, tokens)
			progress := float32(idx+len(e.emitted)) / float32(len(tokens))

			// If we are behind our progress target, attempt to catch up by
			// emitting tokens
			if progress < progressTarget {
				// Compute the approximate number of tokens to emit to reach the
				// target
				toEmit := int(e.tokensPerWindow * (progressTarget - progress) / progressTarget)

				if n, err := e.emitTokens(outCh, toEmit); err != nil && len(tokens) >= e.ngram {
					// If there is no match on currently emitted tokens, an
					// incorrect sequence was likely emitted. Attempt to recover
					// by finding a reasonable way to carry on. To avoid edge
					// cases, also ensure there are at least e.ngram tokens
					// before proceeding.
					toEmit -= n

					// If idx is negative, there was a partially overlapping
					// match. Emit the tokens directly after the overlap.
					for ; idx < 0; idx++ {
						e.emitToken(outCh, tokens[idx+e.ngram])
						toEmit--
					}

					// Overwrite emitted as if we emitted the corrected tokens
					// in the first place
					e.emitted = tokens[idx : idx+e.ngram]

					if _, err := e.emitTokens(outCh, toEmit); err != nil {
						// This should never happen because e.emitted was taken
						// from the current window, and the chain should already
						// be trained on those exact tokens
						return err
					}
				}
			}
		}
	}

	// TODO: Flush remaining tokens
	return nil
}

func (e *Emitter) emitTokens(ch chan<- string, toEmit int) (int, error) {
	var emitted int

	for i := 0; i < toEmit; i++ {
		tok, err := e.q.PredictNext(padLeft(e.emitted, gomarkov.StartToken, e.ngram))
		if err != nil {
			return emitted, err
		}
		if tok == gomarkov.EndToken {
			return emitted, nil
		}

		e.emitToken(ch, tok)
		emitted++
	}

	return emitted, nil
}

func (e *Emitter) emitToken(ch chan<- string, tok string) {
	ch <- tok

	if len(e.emitted) < e.ngram {
		e.emitted = append(e.emitted, tok)
		return
	}

	copy(e.emitted, e.emitted[1:])
	e.emitted[len(e.emitted)-1] = tok
}

// Returns the index in tokens at which the elements of term most closely match.
// A negative index is returned if the closest match does not fully overlap. For
// example, fuzzySearch(["tampa", "atlanta", "chicago"], ["atlanta", "chicago",
// "newyork"]) would return -1.
func fuzzySearch(term, tokens []string) int {
	var idx int
	minDistance := math.MaxInt

	for i := -len(term); i < len(tokens)-len(term); i++ {
		var distance int

		for j := 0; j < len(term); j++ {
			k := i + j

			// If there is no element in token to compare against the current
			// term element, use an arbitrary distance value.
			//
			// TODO: Make this smarter.
			if k < 0 {
				distance += uncertaintyPenalty
			} else {
				distance += fuzzy.LevenshteinDistance(term[j], tokens[k])
			}
		}

		if distance < minDistance {
			minDistance = distance
			idx = i
		}
	}

	return idx
}

func padLeft(toPad []string, padWith string, padLen int) []string {
	if len(toPad) >= padLen {
		return toPad
	}

	padded := make([]string, padLen)
	padding := padLen - len(toPad)
	copy(padded[padding:], toPad)

	for i := 0; i < padding; i++ {
		padded[i] = padWith
	}

	return padded
}

func sanitize(s string) string {
	replaced := withinBracketsPattern.ReplaceAllString(s, "")
	trimmed := strings.TrimSpace(replaced)
	alphanumeric := nonAlphanumericPattern.ReplaceAllString(trimmed, "")
	lowered := strings.ToLower(alphanumeric)

	return spacesPattern.ReplaceAllString(lowered, " ")
}

func tokenize(s string) []string {
	return strings.Split(s, " ")
}
