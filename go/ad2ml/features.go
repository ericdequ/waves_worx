// Package ad2ml turns Analog Discovery 2 sample buffers into compact,
// deterministic feature records suitable for Go ML and Ric EPU adapters.
//
// It intentionally stays stdlib-only. Hardware capture (WaveForms/libdwf via
// cgo or purego), Gonum FFTs, and GoMLX tensor execution should sit on top of
// this package instead of becoming required dependencies for the core.
package ad2ml

import (
	"errors"
	"math"
)

const (
	// DefaultBaseHz mirrors the JS waves_worx modem base tone.
	DefaultBaseHz = 1200
	// DefaultStepHz mirrors the JS waves_worx modem tone spacing.
	DefaultStepHz = 80
	// DefaultSymbolCount is one nibble: 16 FSK tones.
	DefaultSymbolCount = 16
)

// Config describes the tone bank used to convert samples into a feature map.
type Config struct {
	BaseHz      float64
	StepHz      float64
	SymbolCount int
}

// DefaultConfig returns the waves_worx audio-band FSK tone bank.
func DefaultConfig() Config {
	return Config{
		BaseHz:      DefaultBaseHz,
		StepHz:      DefaultStepHz,
		SymbolCount: DefaultSymbolCount,
	}
}

func (c Config) normalized() Config {
	if c.BaseHz <= 0 {
		c.BaseHz = DefaultBaseHz
	}
	if c.StepHz <= 0 {
		c.StepHz = DefaultStepHz
	}
	if c.SymbolCount <= 0 {
		c.SymbolCount = DefaultSymbolCount
	}
	return c
}

// SymbolFreqs returns the candidate FSK tones for a config.
func SymbolFreqs(config Config) []float64 {
	cfg := config.normalized()
	out := make([]float64, cfg.SymbolCount)
	for i := range out {
		out[i] = cfg.BaseHz + float64(i)*cfg.StepHz
	}
	return out
}

// FeatureRecord is the Go Pidar feature map: cheap classical grounding before
// heavier model/vector work.
type FeatureRecord struct {
	RMS          float64   `json:"rms"`
	PeakHz       float64   `json:"peakHz"`
	PeakSymbol   int       `json:"peakSymbol"`
	CentroidHz   float64   `json:"centroidHz"`
	BandEnergies []float64 `json:"bandEnergies"`
	SampleRate   float64   `json:"sampleRate"`
	SampleCount  int       `json:"sampleCount"`
}

// GoertzelPower measures one target frequency in a sample window. It is a
// single-bin DFT, cheap enough for 16-tone classification without a full FFT.
func GoertzelPower(samples []float64, freqHz, sampleRate float64) float64 {
	n := len(samples)
	if n == 0 || sampleRate <= 0 || freqHz <= 0 {
		return 0
	}
	omega := 2 * math.Pi * freqHz / sampleRate
	coeff := 2 * math.Cos(omega)
	var s1, s2 float64
	for _, sample := range samples {
		s0 := sample + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	power := s1*s1 + s2*s2 - coeff*s1*s2
	return power / float64(n*n)
}

// SenseFeatures converts a sample buffer into a fixed feature record. It mirrors
// AD2/src/sense.js so JS and Go can compare the same signal shape.
func SenseFeatures(samples []float64, sampleRate float64, config Config) FeatureRecord {
	cfg := config.normalized()
	freqs := SymbolFreqs(cfg)
	n := len(samples)

	var sumSq float64
	for _, sample := range samples {
		sumSq += sample * sample
	}
	rms := 0.0
	if n > 0 {
		rms = math.Sqrt(sumSq / float64(n))
	}

	bands := make([]float64, len(freqs))
	peak := 0
	for i, freq := range freqs {
		bands[i] = GoertzelPower(samples, freq, sampleRate)
		if i == 0 || bands[i] > bands[peak] {
			peak = i
		}
	}

	var totalEnergy float64
	for _, energy := range bands {
		totalEnergy += energy
	}
	var centroid float64
	if totalEnergy > 0 {
		for i, freq := range freqs {
			centroid += freq * (bands[i] / totalEnergy)
		}
	}

	peakHz := 0.0
	if len(freqs) > 0 {
		peakHz = freqs[peak]
	}
	return FeatureRecord{
		RMS:          rms,
		PeakHz:       peakHz,
		PeakSymbol:   peak,
		CentroidHz:   centroid,
		BandEnergies: bands,
		SampleRate:   sampleRate,
		SampleCount:  n,
	}
}

// NormalizedBands returns a unit-sum copy of BandEnergies. It is the smallest
// tensor-friendly view for prototype classifiers and later GoMLX adapters.
func (f FeatureRecord) NormalizedBands() []float64 {
	out := make([]float64, len(f.BandEnergies))
	var total float64
	for _, energy := range f.BandEnergies {
		total += energy
	}
	if total <= 0 {
		return out
	}
	for i, energy := range f.BandEnergies {
		out[i] = energy / total
	}
	return out
}

// FeatureVector32 packs stable scalar features + normalized bands into float32.
// Shape: [rms, peakSymbol, centroidHz, band0..bandN].
func (f FeatureRecord) FeatureVector32() []float32 {
	bands := f.NormalizedBands()
	out := make([]float32, 0, 3+len(bands))
	out = append(out, float32(f.RMS), float32(f.PeakSymbol), float32(f.CentroidHz))
	for _, band := range bands {
		out = append(out, float32(band))
	}
	return out
}

// CosineSimilarity compares feature vectors or prototype vectors.
func CosineSimilarity(left, right []float64) float64 {
	n := len(left)
	if len(right) < n {
		n = len(right)
	}
	var dot, lm, rm float64
	for i := 0; i < n; i++ {
		dot += left[i] * right[i]
		lm += left[i] * left[i]
		rm += right[i] * right[i]
	}
	if lm <= 0 || rm <= 0 {
		return 0
	}
	return dot / (math.Sqrt(lm) * math.Sqrt(rm))
}

// Prototype is a tiny classical classifier target. It is useful before a real
// trained model exists: compare current band energies against known patterns.
type Prototype struct {
	Label string
	Bands []float64
}

// Classification is the best prototype match for a feature record.
type Classification struct {
	Label  string
	Score  float64
	Index  int
	Reason string
}

// ClassifyByPrototype matches normalized band energies against prototypes.
func ClassifyByPrototype(features FeatureRecord, prototypes []Prototype) (Classification, error) {
	if len(prototypes) == 0 {
		return Classification{}, errors.New("at least one prototype required")
	}
	bands := features.NormalizedBands()
	best := Classification{Index: -1, Score: -1}
	for i, prototype := range prototypes {
		score := CosineSimilarity(bands, prototype.Bands)
		if score > best.Score {
			best = Classification{
				Label:  prototype.Label,
				Score:  score,
				Index:  i,
				Reason: "cosine similarity over normalized AD2 band energies",
			}
		}
	}
	return best, nil
}

// OneHotPrototype returns a prototype for one dominant tone symbol.
func OneHotPrototype(label string, symbol, count int) Prototype {
	if count <= 0 {
		count = DefaultSymbolCount
	}
	bands := make([]float64, count)
	if symbol >= 0 && symbol < count {
		bands[symbol] = 1
	}
	return Prototype{Label: label, Bands: bands}
}

// SineSamples synthesizes a single tone. Tests use it as the Go mirror of the
// JS mock AD2 device; callers with hardware should feed real captured samples.
func SineSamples(freqHz, sampleRate, durationMs, amplitude float64) []float64 {
	if sampleRate <= 0 || durationMs <= 0 {
		return nil
	}
	n := int(math.Round(sampleRate * durationMs / 1000))
	out := make([]float64, n)
	step := 2 * math.Pi * freqHz / sampleRate
	for i := range out {
		out[i] = amplitude * math.Sin(step*float64(i))
	}
	return out
}
