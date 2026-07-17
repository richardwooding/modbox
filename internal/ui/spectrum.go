package ui

import "math"

const (
	fftSize      = 1024
	spectrumBars = 48
)

// spectrum turns the master-mix scope window into smoothed, log-spaced
// frequency bars for the demo-mode analyzer.
type spectrum struct {
	bars  []float32 // smoothed, 0..1
	re    []float64
	im    []float64
	hann  []float64
	edges []int // bar -> first FFT bin, log spaced
}

func newSpectrum() *spectrum {
	s := &spectrum{
		bars: make([]float32, spectrumBars),
		re:   make([]float64, fftSize),
		im:   make([]float64, fftSize),
		hann: make([]float64, fftSize),
	}
	for i := range s.hann {
		s.hann[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}
	// Log-spaced bar edges over bins 1..fftSize/2 (~43 Hz to Nyquist).
	s.edges = make([]int, spectrumBars+1)
	for b := 0; b <= spectrumBars; b++ {
		bin := math.Pow(float64(fftSize/2), float64(b)/float64(spectrumBars))
		s.edges[b] = max(1, int(bin))
	}
	return s
}

// Update feeds one window of mono samples (len >= fftSize; the tail is used).
func (s *spectrum) Update(mono []float32) {
	if len(mono) < fftSize {
		return
	}
	tail := mono[len(mono)-fftSize:]
	for i := range fftSize {
		s.re[i] = float64(tail[i]) * s.hann[i]
		s.im[i] = 0
	}
	fft(s.re, s.im)

	for b := range spectrumBars {
		lo, hi := s.edges[b], s.edges[b+1]
		if hi <= lo {
			hi = lo + 1
		}
		var pk float64
		for bin := lo; bin < hi && bin < fftSize/2; bin++ {
			mag := math.Hypot(s.re[bin], s.im[bin]) / (fftSize / 4)
			if mag > pk {
				pk = mag
			}
		}
		// Rough dB mapping into 0..1 with a 48 dB floor.
		v := float32(0)
		if pk > 0 {
			v = float32(1 + math.Log10(pk)/2.4)
		}
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		if v > s.bars[b] {
			s.bars[b] = v // instant attack
		} else {
			s.bars[b] *= 0.88 // smooth decay
		}
	}
}

// fft is an in-place iterative radix-2 Cooley-Tukey transform.
// len(re) == len(im) must be a power of two.
func fft(re, im []float64) {
	n := len(re)
	// bit-reversal permutation
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		ang := -2 * math.Pi / float64(size)
		wr, wi := math.Cos(ang), math.Sin(ang)
		for start := 0; start < n; start += size {
			cwr, cwi := 1.0, 0.0
			for k := start; k < start+size/2; k++ {
				m := k + size/2
				tr := re[m]*cwr - im[m]*cwi
				ti := re[m]*cwi + im[m]*cwr
				re[m], im[m] = re[k]-tr, im[k]-ti
				re[k], im[k] = re[k]+tr, im[k]+ti
				cwr, cwi = cwr*wr-cwi*wi, cwr*wi+cwi*wr
			}
		}
	}
}
