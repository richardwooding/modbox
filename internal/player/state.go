package player

import "sync"

// stateEvent is the machine's position at the moment a tick's audio began,
// keyed by that audio's absolute first-sample index.
type stateEvent struct {
	startSample      int64
	order, row, tick int
	bpm, speed       int
	vu               []float32 // per-channel tick volume
}

// Snapshot is what the UI reads once per frame.
type Snapshot struct {
	Playing   bool
	Finished  bool
	Order     int
	Row       int
	Tick      int
	BPM       int
	Speed     int
	ChannelVU []float32
	Playhead  int64 // audible sample position
	Underruns int64
	Buffered  int
}

// stateQueue holds pending position events and the current (audible) one.
type stateQueue struct {
	mu     sync.Mutex
	events []stateEvent
	cur    stateEvent
}

func (q *stateQueue) Push(ev stateEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.events = append(q.events, ev)
}

// AtSample advances the current event to the newest one whose audio has
// started playing (startSample <= playhead) and returns it.
func (q *stateQueue) AtSample(playhead int64) stateEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := 0
	for ; i < len(q.events) && q.events[i].startSample <= playhead; i++ {
		q.cur = q.events[i]
	}
	if i > 0 {
		q.events = append(q.events[:0], q.events[i:]...)
	}
	return q.cur
}

// Flush drops pending events (seek); the current event stays until new audio
// overtakes it.
func (q *stateQueue) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.events = q.events[:0]
}
