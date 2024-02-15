package predict

import (
	"github.com/craigrmccown/windowed-markov/utility"
	"github.com/mb-14/gomarkov"
)

var (
	rng = ZeroRng{}
)

type Window struct {
	Tokens []string
	State  *gomarkov.Chain
}

type WindowQ struct {
	q     *utility.Queue[*Window]
	ngram int
}

func NewWindowQ(depth, ngram int) *WindowQ {
	return &WindowQ{
		q:     utility.NewQueue[*Window](depth),
		ngram: ngram,
	}
}

func (w *WindowQ) PushWindow(tokens []string) {
	w.q.Push(&Window{
		Tokens: tokens,
		State:  gomarkov.NewChain(w.ngram),
	})

	// Training step, update markov chains
	w.q.Do(func(window *Window) {
		window.State.Add(tokens)
	})
}

func (w *WindowQ) PredictNext(tokens []string) (string, error) {
	return w.q.Peek().State.GenerateDeterministic(tokens, rng)
}
