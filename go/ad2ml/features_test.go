package ad2ml

import (
	"math"
	"testing"
)

const sampleRate = 48000

func TestGoertzelPowerFindsTone(t *testing.T) {
	freqs := SymbolFreqs(DefaultConfig())
	samples := SineSamples(freqs[10], sampleRate, 80, 1)

	bestSymbol := -1
	bestPower := -1.0
	for symbol, freq := range freqs {
		power := GoertzelPower(samples, freq, sampleRate)
		if power > bestPower {
			bestPower = power
			bestSymbol = symbol
		}
	}

	if bestSymbol != 10 {
		t.Fatalf("best symbol = %d, want 10", bestSymbol)
	}
	if bestPower <= 0.20 {
		t.Fatalf("best power too small: %f", bestPower)
	}
}

func TestSenseFeaturesMirrorsPidarFeatureMap(t *testing.T) {
	freqs := SymbolFreqs(DefaultConfig())
	samples := SineSamples(freqs[12], sampleRate, 120, 1)

	features := SenseFeatures(samples, sampleRate, DefaultConfig())

	if features.PeakSymbol != 12 {
		t.Fatalf("PeakSymbol = %d, want 12", features.PeakSymbol)
	}
	if features.PeakHz != freqs[12] {
		t.Fatalf("PeakHz = %f, want %f", features.PeakHz, freqs[12])
	}
	if features.RMS < 0.5 || features.RMS > 1.0 {
		t.Fatalf("RMS = %f, want sine-like range", features.RMS)
	}
	if math.Abs(features.CentroidHz-freqs[12]) > 80 {
		t.Fatalf("CentroidHz = %f, want near %f", features.CentroidHz, freqs[12])
	}
	if len(features.BandEnergies) != DefaultSymbolCount {
		t.Fatalf("BandEnergies len = %d, want %d", len(features.BandEnergies), DefaultSymbolCount)
	}
}

func TestNormalizedBandsAndFeatureVector32(t *testing.T) {
	freqs := SymbolFreqs(DefaultConfig())
	features := SenseFeatures(SineSamples(freqs[3], sampleRate, 120, 1), sampleRate, DefaultConfig())

	bands := features.NormalizedBands()
	var total float64
	for _, value := range bands {
		total += value
	}
	if math.Abs(total-1) > 1e-9 {
		t.Fatalf("normalized bands total = %.12f, want 1", total)
	}

	vector := features.FeatureVector32()
	if len(vector) != 3+DefaultSymbolCount {
		t.Fatalf("FeatureVector32 len = %d, want %d", len(vector), 3+DefaultSymbolCount)
	}
	if vector[1] != 3 {
		t.Fatalf("feature vector peak symbol = %f, want 3", vector[1])
	}
}

func TestClassifyByPrototype(t *testing.T) {
	freqs := SymbolFreqs(DefaultConfig())
	features := SenseFeatures(SineSamples(freqs[7], sampleRate, 100, 1), sampleRate, DefaultConfig())

	got, err := ClassifyByPrototype(features, []Prototype{
		OneHotPrototype("low-tone", 2, DefaultSymbolCount),
		OneHotPrototype("target-tone", 7, DefaultSymbolCount),
		OneHotPrototype("high-tone", 14, DefaultSymbolCount),
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if got.Label != "target-tone" {
		t.Fatalf("label = %q, want target-tone", got.Label)
	}
	if got.Score < 0.9 {
		t.Fatalf("score = %f, want strong match", got.Score)
	}
}

func TestSilenceHasZeroEnergy(t *testing.T) {
	features := SenseFeatures(make([]float64, 256), sampleRate, DefaultConfig())
	if features.RMS != 0 {
		t.Fatalf("RMS = %f, want 0", features.RMS)
	}
	if features.CentroidHz != 0 {
		t.Fatalf("CentroidHz = %f, want 0", features.CentroidHz)
	}
	for i, energy := range features.BandEnergies {
		if energy != 0 {
			t.Fatalf("band %d = %f, want 0", i, energy)
		}
	}
}
