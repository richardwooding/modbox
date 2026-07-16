package player

import (
	"io"
	"sync"
)

// pcmRing is the bridge between the render goroutine (which produces s16le
// stereo PCM in ~20ms tick chunks) and ebiten's audio player (which pulls).
//
// Write blocks when the ring is full — that backpressure is what stops the
// renderer free-running ahead of playback. Read never blocks: it hands back
// silence on underrun (the audio device must not stall) and io.EOF once the
// song is done and the ring has drained.
type pcmRing struct {
	mu      sync.Mutex
	notFull sync.Cond

	buf  []byte
	r, w int
	n    int // bytes currently buffered

	played    int64 // samples handed to the reader (incl. silence)
	offset    int64 // timeline correction: rendered-sample index minus played, adjusted on flush/underrun
	underruns int64 // silence insertions (debug overlay)

	closed  bool // no more writes; drain then EOF
	flushed bool // wake a blocked writer whose data was just discarded
}

func newPCMRing(capBytes int) *pcmRing {
	p := &pcmRing{buf: make([]byte, capBytes)}
	p.notFull.L = &p.mu
	return p
}

// Write appends pcm, blocking while the ring is full. It returns false if the
// ring was closed or flushed while waiting (the caller should re-check state).
func (p *pcmRing) Write(pcm []byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(pcm) > 0 {
		for p.n == len(p.buf) && !p.closed && !p.flushed {
			p.notFull.Wait()
		}
		if p.closed {
			return false
		}
		if p.flushed {
			p.flushed = false
			return false
		}
		chunk := min(len(pcm), len(p.buf)-p.n)
		// copy into the ring, possibly wrapping
		first := min(chunk, len(p.buf)-p.w)
		copy(p.buf[p.w:], pcm[:first])
		copy(p.buf, pcm[first:chunk])
		p.w = (p.w + chunk) % len(p.buf)
		p.n += chunk
		pcm = pcm[chunk:]
	}
	return true
}

// Read implements io.Reader for the audio player. Short reads are fine;
// full-length silence is returned on underrun.
func (p *pcmRing) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Frame-align the destination so played stays in whole samples.
	want := len(b) / BytesPerFrame * BytesPerFrame
	if want == 0 {
		return 0, nil
	}

	if p.n == 0 {
		if p.closed {
			return 0, io.EOF
		}
		// Underrun: silence, and pull the timeline mapping back so the
		// UI doesn't run ahead of what is audible.
		for i := range want {
			b[i] = 0
		}
		p.played += int64(want / BytesPerFrame)
		p.offset -= int64(want / BytesPerFrame)
		p.underruns++
		return want, nil
	}

	got := min(want, p.n/BytesPerFrame*BytesPerFrame)
	first := min(got, len(p.buf)-p.r)
	copy(b, p.buf[p.r:p.r+first])
	copy(b[first:got], p.buf)
	p.r = (p.r + got) % len(p.buf)
	p.n -= got
	p.played += int64(got / BytesPerFrame)
	p.notFull.Signal()
	return got, nil
}

// PlayheadSample maps the reader's consumption back onto the renderer's
// sample timeline (flushes and underruns shift the mapping via offset).
func (p *pcmRing) PlayheadSample() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.played + p.offset
}

// Flush discards buffered audio (seek) and realigns the timeline mapping so
// the next written sample — renderedSamples — is what plays next.
func (p *pcmRing) Flush(renderedSamples int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.r, p.w, p.n = 0, 0, 0
	p.offset = renderedSamples - p.played
	p.flushed = true
	p.notFull.Signal()
}

// Close marks the stream complete; readers drain then get io.EOF.
func (p *pcmRing) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.notFull.Broadcast()
}

// Underruns reports how many silence insertions have happened.
func (p *pcmRing) Underruns() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.underruns
}

// Buffered reports bytes currently in the ring (debug overlay).
func (p *pcmRing) Buffered() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.n
}
