package utility

// Queue is a constant-memory queue implementation that overwrites the last
// inserted element when full
type Queue[T any] struct {
	buf       []T
	len, head int
}

func NewQueue[T any](cap int) *Queue[T] {
	return &Queue[T]{
		buf:  make([]T, cap),
		head: cap - 1,
	}
}

func (q *Queue[T]) Cap() int {
	return len(q.buf)
}

func (q *Queue[T]) Len() int {
	return q.len
}

// Push adds an element to the queue. If the queue is at capacity, the last
// inserted element is overwritten
func (q *Queue[T]) Push(el T) {
	// Perform bookkeeping
	q.head = (q.head + 1) % q.Cap()

	if q.Len() < q.Cap() {
		q.len++
	}

	// Overwrite next element
	q.buf[q.head] = el
}

// Do calls fn for every element from last to first in order of insertion
func (q *Queue[T]) Do(fn func(T)) {
	for i := 0; i < q.Len(); i++ {
		fn(q.buf[(i+q.head)%q.Len()])
	}
}

// Peek returns the last inserted element. If empty, a default-initialized value
// is returned
func (q *Queue[T]) Peek() T {
	return q.buf[q.head]
}

func (q *Queue[T]) Full() bool {
	return q.Len() == q.Cap()
}
