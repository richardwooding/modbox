package ui

import (
	"math"
	"testing"
)

func TestFFTFindsSine(t *testing.T) {
	re := make([]float64, fftSize)
	im := make([]float64, fftSize)
	const bin = 100 // exact-bin sine, no leakage
	for i := range re {
		re[i] = math.Sin(2 * math.Pi * float64(bin) * float64(i) / fftSize)
	}
	fft(re, im)

	peak, peakBin := 0.0, 0
	for b := 1; b < fftSize/2; b++ {
		if mag := math.Hypot(re[b], im[b]); mag > peak {
			peak, peakBin = mag, b
		}
	}
	if peakBin != bin {
		t.Fatalf("peak at bin %d, want %d", peakBin, bin)
	}
	// A unit sine concentrates N/2 magnitude in its bin.
	if peak < fftSize/2*0.9 {
		t.Errorf("peak magnitude %f too small", peak)
	}
}

func TestSpectrumRespondsToSignal(t *testing.T) {
	s := newSpectrum()
	mono := make([]float32, fftSize)
	for i := range mono {
		mono[i] = float32(math.Sin(2*math.Pi*float64(i)/32) * 0.8) // ~1.4kHz at 44.1k
	}
	s.Update(mono)

	var energy float32
	for _, b := range s.bars {
		energy += b
	}
	if energy == 0 {
		t.Fatal("spectrum flat for a strong sine")
	}

	// Silence must decay the bars, not hold them.
	before := energy
	for range 60 {
		s.Update(make([]float32, fftSize))
	}
	var after float32
	for _, b := range s.bars {
		after += b
	}
	if after >= before {
		t.Errorf("bars did not decay: before %f, after %f", before, after)
	}
}
