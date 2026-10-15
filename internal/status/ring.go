package status

// ring is a fixed-capacity FIFO that overwrites the oldest element.
type ring[T any] struct {
	buf  []T
	head int // index of the oldest element
	n    int
}

func newRing[T any](capacity int) *ring[T] {
	return &ring[T]{buf: make([]T, capacity)}
}

func (r *ring[T]) push(v T) {
	if r.n < len(r.buf) {
		r.buf[(r.head+r.n)%len(r.buf)] = v
		r.n++
		return
	}
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
}

// items returns a copy, oldest first.
func (r *ring[T]) items() []T {
	out := make([]T, r.n)
	for i := 0; i < r.n; i++ {
		out[i] = r.buf[(r.head+i)%len(r.buf)]
	}
	return out
}

func (r *ring[T]) last() (T, bool) {
	var zero T
	if r.n == 0 {
		return zero, false
	}
	return r.buf[(r.head+r.n-1)%len(r.buf)], true
}
