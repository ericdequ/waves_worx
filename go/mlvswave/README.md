# mlvswave — ML vs wave-compute, measured

A reproducible experiment: on the AD2 tone-classification task, does a **trained
ML model** beat **classical wave-compute** (Goertzel DSP)? It runs on CPU
anywhere (this laptop included) and is the harness you scale onto a real GPU.

```bash
env GOCACHE=/tmp/go-build-cache go -C .. test -v ./mlvswave   # the accuracy table
env GOCACHE=/tmp/go-build-cache go -C .. test -bench . ./mlvswave  # ns/op, both paths
```

## What it shows

Both paths consume the same captured samples. Wave-compute runs Goertzel over the
16 tones and takes the argmax. The ML path (a small softmax trained with SGD) is
fed the *same* band energies and learns to classify them.

The measured result — and the reason this is worth a test, not an assumption:

- **Clean signal:** wave-compute is ~perfect with zero training.
- **Every SNR:** ML *ties at best* — because argmax of the band energies is
  already a *sufficient statistic*, and you can't out-learn that. ML is also
  strictly more expensive (it needs the same Goertzel features **first**, then a
  matmul — see the two benchmarks).
- **The lesson:** when the feature is analytically known (a known tone bank),
  classical wave-compute wins. ML earns its keep only where **no closed-form
  statistic exists** — open-ended "vibe", unknown/learned channels, multipath, or
  features you can't write down. That's exactly BEV's "ground classically first,
  defer the vectors" thesis, now with a number behind it.

## Setup for real GPU ML (RX 480 — read this first)

The RX 480 is a **desktop PCIe card** — it goes in the i7 build, **not** a laptop
or a Pi. Once it's in a machine, the honest Polaris (gfx803) reality on Fedora:

- **ROCm dropped official Polaris support.** Modern ROCm targets newer GCN/RDNA.
  You can sometimes force it with `HSA_OVERRIDE_GFX_VERSION=8.0.3`, but it's
  fragile and unsupported.
- **The practical 2026 path is Vulkan compute** — `llama.cpp`'s Vulkan backend
  runs well on Polaris via Mesa RADV, no ROCm needed. Good for inference demos.
- **OpenCL** (rusticl/clover or rocm-opencl) works for some kernels; modern
  PyTorch no longer uses it.
- Raw FP32 is fine (~5.8 TFLOPS) but there are **no FP16/tensor units**, so it's
  a weak *modern-ML* card regardless of stack.

This package deliberately needs **none of that** to run — it's the CPU control.
To scale the ML side onto the GPU, keep this experiment's interface and swap the
classifier for a **GoMLX** model (Vulkan/ROCm backend) or shell out to a
`llama.cpp`/ONNX runtime; the dataset, the wave baseline, and the accuracy/latency
harness stay identical, so the comparison remains apples-to-apples.

## Files

- `dataset.go` — synthesize labeled noisy tone windows (deterministic).
- `classify.go` — `WaveClassify` (Goertzel argmax) + `LinearSoftmax` (trained).
- `mlvswave_test.go` — the accuracy table + the two latency benchmarks.

## References

- G. Goertzel, *Amer. Math. Monthly* 65 (1958) — the single-bin DFT the
  wave-compute classifier uses.
- The "sufficient statistic" result: for a tone in symmetric noise, argmax of the
  Goertzel band energies is the Bayes-optimal decision — so a learned model fed
  the same features can *tie* but not beat it (the measured outcome in the test).

## See also

- [`../ad2ml`](../ad2ml) — the feature layer both paths consume
- [`../../AD2/wavecompute`](../../AD2/wavecompute) — the broader wave-native kernels
