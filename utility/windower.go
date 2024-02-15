package utility

type Windower[T any] struct {
	step int
	buf  []T
}

func NewWindower[T any](width, step int) *Windower[T] {
	if width <= 0 || step <= 0 {
		panic("Width and step must be nonzero, positive integers")
	}

	return &Windower[T]{
		step: step,
		buf:  make([]T, width),
	}
}

func (b *Windower[T]) Do(inCh <-chan []T, outCh chan<- []T) {
	defer close(outCh)

	remaining := b.step

	process := func(chunk []T) {
		for len(chunk) > 0 {
			skip := remaining - len(b.buf)

			if skip >= len(chunk) {
				remaining -= len(chunk)
				chunk = nil
				continue
			}

			if skip > 0 {
				remaining -= skip
				chunk = chunk[skip:]
			}

			copied := copy(b.buf[len(b.buf)-remaining:], chunk)
			remaining -= copied
			chunk = chunk[copied:]

			if remaining == 0 {
				cpy := make([]T, len(b.buf))
				copy(cpy, b.buf)
				outCh <- cpy

				remaining = b.step
				if len(b.buf) > remaining {
					copy(b.buf, b.buf[remaining:])
				}
			}
		}
	}

	for chunk := range inCh {
		process(chunk)
	}

	process(make([]T, len(b.buf)))
}
