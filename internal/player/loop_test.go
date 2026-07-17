package player

import (
	"io"
	"testing"
	"time"
)

// tinyMOD is 64 rows at BPM 125 / speed 6 ≈ 7.7s of audio when played once.
const tinyMODSamples = int64(64 * 6 * (SampleRate * 25 / 10 / 125)) // rows * ticks * samples-per-tick

func drainUntil(t *testing.T, p *Player, playhead int64, timeout time.Duration) bool {
	t.Helper()
	buf := make([]byte, 65536)
	deadline := time.Now().Add(timeout)
	for p.Snapshot().Playhead < playhead {
		if _, err := p.Stream().Read(buf); err == io.EOF {
			return false
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out draining stream")
		}
	}
	return true
}

func TestLoopRestartsSong(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.SetLoop(true)
	p.Start()

	// Play past 1.5 song lengths — without looping this hits EOF first.
	if !drainUntil(t, p, tinyMODSamples*3/2, 60*time.Second) {
		t.Fatal("stream ended although loop was enabled")
	}
	if p.Snapshot().Finished {
		t.Fatal("player reported finished in loop mode")
	}
}

func TestNoLoopEndsSong(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.Start()

	if drainUntil(t, p, tinyMODSamples*3/2, 60*time.Second) {
		t.Fatal("stream kept going past song end with loop disabled")
	}
}

func TestMuteSilencesChannel(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.CanMute() {
		t.Fatal("machine does not support muting")
	}
	p.ToggleMute(0) // channel 0 carries the row-0 note
	if !p.IsMuted(0) {
		t.Fatal("channel 0 not marked muted")
	}
	p.Start()
	drainUntil(t, p, SampleRate/4, 30*time.Second)

	scope := make([]float32, 2048)
	p.Scope(0, scope)
	for _, v := range scope {
		if v != 0 {
			t.Fatalf("muted channel 0 has scope energy %f", v)
		}
	}
}

func TestSoloIsolatesChannel(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.Solo(2)
	for ch, want := range []bool{true, true, false, true} {
		if p.IsMuted(ch) != want {
			t.Errorf("after Solo(2): IsMuted(%d) = %v, want %v", ch, p.IsMuted(ch), want)
		}
	}
	// Solo again on the same channel un-mutes everything.
	p.Solo(2)
	for ch := range 4 {
		if p.IsMuted(ch) {
			t.Errorf("after second Solo(2): channel %d still muted", ch)
		}
	}
}
