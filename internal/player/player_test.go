package player

import (
	"encoding/binary"
	"io"
	"testing"
	"time"
)

// tinyMOD synthesizes a minimal valid 4-channel ProTracker module: one
// pattern, one order, one looped square-wave sample, a note on row 0.
func tinyMOD() []byte {
	buf := make([]byte, 0, 1084+1024+64)

	title := make([]byte, 20)
	copy(title, "modbox test")
	buf = append(buf, title...)

	// 31 sample headers x 30 bytes
	for i := range 31 {
		h := make([]byte, 30)
		if i == 0 {
			copy(h, "square")
			binary.BigEndian.PutUint16(h[22:], 32) // length in words (64 bytes)
			h[24] = 0                              // finetune
			h[25] = 64                             // volume
			binary.BigEndian.PutUint16(h[26:], 0)  // loop start
			binary.BigEndian.PutUint16(h[28:], 32) // loop length in words
		}
		buf = append(buf, h...)
	}

	buf = append(buf, 1)   // song length
	buf = append(buf, 127) // restart position
	orders := make([]byte, 128)
	buf = append(buf, orders...) // order[0] = pattern 0
	buf = append(buf, "M.K."...)

	// pattern 0: 64 rows x 4 channels x 4 bytes
	pattern := make([]byte, 64*4*4)
	// row 0, channel 0: sample 1, period 428 (C-2), no effect
	pattern[0] = 0x01 // sample hi nibble (0) | period hi (0x1)
	pattern[1] = 0xAC // period lo (428 = 0x1AC)
	pattern[2] = 0x10 // sample lo nibble (1) << 4 | effect 0
	pattern[3] = 0x00
	buf = append(buf, pattern...)

	// sample data: 64-byte square wave
	sample := make([]byte, 64)
	for i := range sample {
		if i < 32 {
			sample[i] = 0x40
		} else {
			sample[i] = 0xC0
		}
	}
	return append(buf, sample...)
}

func TestLoadTinyMOD(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	if p.Info.Format != "MOD" {
		t.Errorf("Format = %q, want MOD", p.Info.Format)
	}
	if p.Info.NumChannels != 4 {
		t.Errorf("NumChannels = %d, want 4", p.Info.NumChannels)
	}
	if p.Info.NumOrders != 1 {
		t.Errorf("NumOrders = %d, want 1", p.Info.NumOrders)
	}
	rows, ok := p.Info.PatternText[0]
	if !ok {
		t.Fatal("PatternText missing pattern 0")
	}
	if len(rows) != 64 {
		t.Fatalf("pattern 0 has %d rows, want 64", len(rows))
	}
	if len(rows[0]) != 4 {
		t.Fatalf("row 0 has %d cells, want 4: %q", len(rows[0]), rows[0])
	}
}

func TestRenderProducesPCMAndState(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()
	p.Start()

	// Drain the stream until ~0.5s of real audio has passed the playhead.
	// A tight loop outpaces the renderer, so underrun silence is expected;
	// Snapshot().Playhead counts only real rendered samples.
	buf := make([]byte, 4096)
	nonZero := false
	deadline := time.Now().Add(10 * time.Second)
	for p.Snapshot().Playhead < SampleRate/2 {
		n, err := p.Stream().Read(buf)
		for _, b := range buf[:n] {
			if b != 0 {
				nonZero = true
				break
			}
		}
		if err == io.EOF {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out draining stream")
		}
	}

	// PCM must not be all zeros — the square wave should be audible.
	if !nonZero {
		t.Error("rendered PCM is silent")
	}

	snap := p.Snapshot()
	if snap.Order != 0 {
		t.Errorf("Order = %d, want 0", snap.Order)
	}
	if snap.BPM < 100 || snap.BPM > 150 {
		t.Errorf("BPM = %d, want ~125", snap.BPM)
	}
	if len(snap.ChannelVU) != 4 {
		t.Errorf("ChannelVU has %d channels, want 4", len(snap.ChannelVU))
	}

	// The scope for channel 0 should show signal once audio has played.
	scope := make([]float32, 256)
	p.Scope(0, scope)
	var energy float32
	for _, s := range scope {
		if s < 0 {
			s = -s
		}
		energy += s
	}
	if energy == 0 {
		t.Error("channel 0 scope is flat; expected square-wave energy")
	}
}

func TestSnapshotRowAdvances(t *testing.T) {
	p, err := Load(tinyMOD())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()
	p.Start()

	buf := make([]byte, 4096)
	sawRow := -1
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := p.Stream().Read(buf); err == io.EOF {
			break
		}
		if r := p.Snapshot().Row; r > sawRow {
			sawRow = r
		}
		if sawRow >= 2 {
			return // rows advanced as audio was consumed
		}
	}
	t.Fatalf("row never advanced past %d", sawRow)
}
