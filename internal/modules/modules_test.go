package modules_test

import (
	"io"
	"testing"
	"time"

	"github.com/richardwooding/modbox/internal/modules"
	"github.com/richardwooding/modbox/internal/player"
)

// TestDemosRender proves every bundled demo loads and renders real,
// non-silent audio through the playback engine — the library's XM support is
// young, so nothing ships in the demo list without passing this.
func TestDemosRender(t *testing.T) {
	demos := modules.Demos()
	if len(demos) == 0 {
		t.Fatal("no bundled demos")
	}
	for _, d := range demos {
		t.Run(d.Title, func(t *testing.T) {
			if d.License == "" || d.Source == "" || d.Artist == "" {
				t.Fatalf("demo %q is missing attribution metadata", d.Title)
			}
			p, err := player.Load(d.Data)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			defer p.Close()
			if p.Info.Format != "XM" {
				t.Errorf("Format = %q, want XM", p.Info.Format)
			}
			p.Start()

			buf := make([]byte, 8192)
			nonZero := false
			deadline := time.Now().Add(20 * time.Second)
			for p.Snapshot().Playhead < player.SampleRate*2 { // 2s of audio
				n, err := p.Stream().Read(buf)
				for _, b := range buf[:n] {
					if b != 0 {
						nonZero = true
						break
					}
				}
				if err == io.EOF {
					t.Fatal("song ended before 2s — render failed early")
				}
				if time.Now().After(deadline) {
					t.Fatal("timed out rendering")
				}
			}
			if !nonZero {
				t.Error("rendered 2s of pure silence")
			}
			snap := p.Snapshot()
			if snap.BPM == 0 {
				t.Error("BPM never derived")
			}
			t.Logf("%s: %d channels, %d orders, BPM %d", d.Title, p.Info.NumChannels, p.Info.NumOrders, snap.BPM)
		})
	}
}
