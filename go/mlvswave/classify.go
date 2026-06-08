package mlvswave

import (
	"math"
	"math/rand"

	"github.com/ericdequ/waves_worx/go/ad2ml"
)

// WaveClassify is the classical wave-compute path: Goertzel band energies, then
// argmax tone. Zero parameters, zero training — the tone bank IS the model. For
// a single tone in symmetric noise, argmax of the band energies is the Bayes-
// optimal decision, which is the whole point of the comparison below.
func WaveClassify(samples []float64, sampleRate float64, cfg ad2ml.Config) int {
	return ad2ml.SenseFeatures(samples, sampleRate, cfg).PeakSymbol
}

// LinearSoftmax is the smallest honest learned baseline: multinomial logistic
// regression (one dense layer + softmax), trained with SGD on cross-entropy.
// It consumes the SAME normalized band energies the wave path argmaxes — so it
// can at best LEARN that argmax, never out-reason a sufficient statistic.
type LinearSoftmax struct {
	Classes int
	Dim     int
	W       [][]float64 // [Classes][Dim]
	B       []float64   // [Classes]
}

// NewLinearSoftmax initializes small random weights (deterministic for a seed).
func NewLinearSoftmax(classes, dim int, seed int64) *LinearSoftmax {
	rng := rand.New(rand.NewSource(seed))
	w := make([][]float64, classes)
	for k := range w {
		w[k] = make([]float64, dim)
		for d := range w[k] {
			w[k][d] = (rng.Float64()*2 - 1) * 0.01
		}
	}
	return &LinearSoftmax{Classes: classes, Dim: dim, W: w, B: make([]float64, classes)}
}

// probs returns the numerically-stable softmax distribution over classes.
func (m *LinearSoftmax) probs(x []float64) []float64 {
	logits := make([]float64, m.Classes)
	max := math.Inf(-1)
	for k := 0; k < m.Classes; k++ {
		sum := m.B[k]
		for d := 0; d < m.Dim && d < len(x); d++ {
			sum += m.W[k][d] * x[d]
		}
		logits[k] = sum
		if sum > max {
			max = sum
		}
	}
	var z float64
	for k := range logits {
		logits[k] = math.Exp(logits[k] - max)
		z += logits[k]
	}
	for k := range logits {
		logits[k] /= z
	}
	return logits
}

// Predict returns the argmax class.
func (m *LinearSoftmax) Predict(x []float64) int {
	p := m.probs(x)
	best := 0
	for k := 1; k < len(p); k++ {
		if p[k] > p[best] {
			best = k
		}
	}
	return best
}

// Train runs SGD over the dataset for the given epochs and learning rate.
// Gradient of softmax cross-entropy is simply (p - onehot) ⊗ x.
func (m *LinearSoftmax) Train(data []Sample, epochs int, lr float64, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	order := make([]int, len(data))
	for i := range order {
		order[i] = i
	}
	for e := 0; e < epochs; e++ {
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		for _, i := range order {
			s := data[i]
			p := m.probs(s.Features)
			for k := 0; k < m.Classes; k++ {
				grad := p[k]
				if k == s.Symbol {
					grad -= 1
				}
				for d := 0; d < m.Dim && d < len(s.Features); d++ {
					m.W[k][d] -= lr * grad * s.Features[d]
				}
				m.B[k] -= lr * grad
			}
		}
	}
}

// Accuracy is the fraction of samples a predictor labels correctly.
func Accuracy(predict func(Sample) int, data []Sample) float64 {
	if len(data) == 0 {
		return 0
	}
	hits := 0
	for _, s := range data {
		if predict(s) == s.Symbol {
			hits++
		}
	}
	return float64(hits) / float64(len(data))
}

// WavePredictor adapts WaveClassify to the Accuracy signature.
func WavePredictor(sampleRate float64, cfg ad2ml.Config) func(Sample) int {
	return func(s Sample) int { return WaveClassify(s.Samples, sampleRate, cfg) }
}
