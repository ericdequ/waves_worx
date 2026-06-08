// Package mlvswave benchmarks classical "wave compute" (Goertzel DSP) against a
// learned ML classifier on the same task: recover which FSK tone is present in a
// noisy sample window. It exists to MEASURE — not assume — when a trained model
// is worth its cost versus a sufficient classical statistic.
//
// stdlib-only. The ML side here is a tiny CPU softmax so the comparison runs
// anywhere (this laptop included). Heavy GPU backends (GoMLX on ROCm/Vulkan for
// the RX 480) plug in ABOVE this package without changing the experiment — see
// README.md.
package mlvswave

import (
	"math/rand"

	"github.com/ericdequ/waves_worx/go/ad2ml"
)

// Sample is one labeled tone window plus its precomputed ML feature vector
// (normalized Goertzel band energies — the same features the wave path uses).
type Sample struct {
	Symbol   int       // ground-truth tone index
	Samples  []float64 // raw "captured" samples (the wave-compute input)
	Features []float64 // normalized band energies (the ML input)
}

// GenerateDataset synthesizes n labeled tone windows at a given noise amplitude
// (higher noiseAmp = lower SNR). Deterministic for a fixed seed so the benchmark
// is reproducible. Each window is one tone + uniform noise — the same shape the
// AD2 mock device produces, so JS and Go exercise identical signals.
func GenerateDataset(n int, sampleRate, symbolMs, noiseAmp float64, cfg ad2ml.Config, seed int64) []Sample {
	rng := rand.New(rand.NewSource(seed))
	freqs := ad2ml.SymbolFreqs(cfg)
	out := make([]Sample, n)
	for i := 0; i < n; i++ {
		sym := rng.Intn(len(freqs))
		samples := ad2ml.SineSamples(freqs[sym], sampleRate, symbolMs, 1.0)
		if noiseAmp > 0 {
			for j := range samples {
				samples[j] += (rng.Float64()*2 - 1) * noiseAmp
			}
		}
		out[i] = Sample{
			Symbol:   sym,
			Samples:  samples,
			Features: ad2ml.SenseFeatures(samples, sampleRate, cfg).NormalizedBands(),
		}
	}
	return out
}
