package player

import "sync"

// scopeTap keeps a rolling window of one channel's mono amplitude, indexed by
// absolute rendered-sample position, so the UI can draw an oscilloscope for
// exactly the slice of audio that is currently audible.
type scopeTap struct {
	mu  sync.Mutex
	buf []float32
	pos int64 // absolute sample index of the next write; buf holds the last len(buf) samples
}

func newScopeTap(capacity int) *scopeTap {
	return &scopeTap{buf: make([]float32, capacity)}
}

func (t *scopeTap) Write(samples []float32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range samples {
		t.buf[t.pos%int64(len(t.buf))] = s
		t.pos++
	}
}

// CopyWindow fills out with the samples ending at endSample (exclusive).
// Regions outside what the tap has seen come back as zeros.
func (t *scopeTap) CopyWindow(endSample int64, out []float32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := int64(len(t.buf))
	for i := range out {
		idx := endSample - int64(len(out)) + int64(i)
		if idx < 0 || idx >= t.pos || idx < t.pos-n {
			out[i] = 0
			continue
		}
		out[i] = t.buf[idx%n]
	}
}

// Reset discards tap history (seek).
func (t *scopeTap) Reset(pos int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.buf {
		t.buf[i] = 0
	}
	t.pos = pos
}
