package mlvswave

import (
	"testing"

	"github.com/ericdequ/waves_worx/go/ad2ml"
)

const (
	testSampleRate = 48000.0
	testSymbolMs   = 60.0
)

// TestMLvsWaveAccuracy is the experiment: at several SNRs, compare classical
// wave-compute (Goertzel argmax) against a trained softmax fed the same band
// energies. It prints the table and asserts the measurable insight.
func TestMLvsWaveAccuracy(t *testing.T) {
	cfg := ad2ml.DefaultConfig()
	wave := WavePredictor(testSampleRate, cfg)
	noises := []float64{0.0, 0.3, 0.8, 1.5}

	t.Logf("%-9s %-9s %-9s  %s", "noiseAmp", "wave_acc", "ml_acc", "verdict")
	for _, noise := range noises {
		train := GenerateDataset(2000, testSampleRate, testSymbolMs, noise, cfg, 1)
		test := GenerateDataset(500, testSampleRate, testSymbolMs, noise, cfg, 99)

		model := NewLinearSoftmax(cfg.SymbolCount, cfg.SymbolCount, 7)
		model.Train(train, 80, 0.5, 7)

		waveAcc := Accuracy(wave, test)
		mlAcc := Accuracy(func(s Sample) int { return model.Predict(s.Features) }, test)
		t.Logf("%-9.1f %-9.3f %-9.3f  %s", noise, waveAcc, mlAcc, verdict(waveAcc, mlAcc))

		// The insight: the model is DOWNSTREAM of the same Goertzel bands, and
		// argmax of those bands is already optimal — so ML cannot meaningfully
		// beat wave-compute on this task. You can't out-learn a sufficient statistic.
		if mlAcc > waveAcc+0.06 {
			t.Errorf("noise %.1f: ML (%.3f) should not beat wave-compute (%.3f)", noise, mlAcc, waveAcc)
		}
		if noise == 0.0 && waveAcc < 0.99 {
			t.Errorf("clean signal: wave-compute should be ~perfect, got %.3f", waveAcc)
		}
	}
	t.Log("conclusion: when the feature is analytically known, wave-compute wins " +
		"(no training, lower cost). ML earns its place only where no closed-form " +
		"statistic exists (open-ended 'vibe', unknown channels) — see README.")
}

func verdict(wave, ml float64) string {
	switch {
	case wave >= ml+0.03:
		return "wave-compute wins (cheaper + better)"
	case ml >= wave+0.03:
		return "ML wins"
	default:
		return "tie — ML only matched the DSP it was fed"
	}
}

// BenchmarkWaveClassify measures one full classical classification: Goertzel
// over 16 tones + argmax, sensing and deciding in one pass.
func BenchmarkWaveClassify(b *testing.B) {
	cfg := ad2ml.DefaultConfig()
	s := ad2ml.SineSamples(ad2ml.SymbolFreqs(cfg)[7], testSampleRate, testSymbolMs, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WaveClassify(s, testSampleRate, cfg)
	}
}

// BenchmarkMLClassify is the honest accounting: the model still needs the SAME
// Goertzel features first, THEN a matmul — strictly more work than wave-compute.
func BenchmarkMLClassify(b *testing.B) {
	cfg := ad2ml.DefaultConfig()
	s := ad2ml.SineSamples(ad2ml.SymbolFreqs(cfg)[7], testSampleRate, testSymbolMs, 1)
	model := NewLinearSoftmax(cfg.SymbolCount, cfg.SymbolCount, 7)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		feats := ad2ml.SenseFeatures(s, testSampleRate, cfg).NormalizedBands()
		_ = model.Predict(feats)
	}
}
