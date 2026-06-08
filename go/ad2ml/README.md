# ad2ml — AD2/Pidar feature helpers for Go ML adapters

`ad2ml` is the stdlib-only Go mirror of `AD2/src/sense.js`: it turns Analog
Discovery 2 voltage samples into compact, deterministic feature records that can
feed future Gonum, GoMLX, Python, or EPU pipelines.

This package deliberately does **not** bind to `libdwf` and does **not** import
Gonum/GoMLX yet. It is the stable middle layer:

```
AD2 / mock samples
  -> ad2ml.SenseFeatures
  -> FeatureVector32 / normalized bands
  -> future Gonum FFTs, GoMLX tensors, EPU vector service, WAP control loop
```

Why start here:

- pure Go and mobile/edge friendly
- unit-testable without AD2 hardware
- deterministic feature shape for `.ric` manifests and EPU parser fixtures
- easy to adapt into Gonum matrices or GoMLX tensors later

Future adapters:

- `ad2dwf`: cgo or `purego` WaveForms capture adapter
- `ad2gonum`: FFT/windowing/filter helpers
- `ad2gomlx`: tensor packing and local inference
- `ad2wap`: digital control-plane loop for wave analog processor experiments

## Run

```bash
env GOCACHE=/tmp/go-build-cache go test ./...
```

## API

| Export | Does |
|---|---|
| `SenseFeatures(samples, sr, cfg)` | samples → `FeatureRecord` (rms, peak, centroid, band energies) |
| `GoertzelPower(samples, f, sr)` | single-frequency power — one DFT bin |
| `FeatureRecord.FeatureVector32()` | tensor-ready `[rms, peakSymbol, centroidHz, bands…]` |
| `ClassifyByPrototype(features, protos)` | nearest prototype by cosine |
| `SineSamples(f, sr, ms, amp)` | synthesize a tone (the JS mock's mirror) |

## References

- G. Goertzel, *Amer. Math. Monthly* 65 (1958) — the single-bin DFT behind
  `GoertzelPower`.
- Mirrors `AD2/src/sense.js`; see [`../../AD2`](../../AD2) for the Pidar framing.

## See also

- [`../mlvswave`](../mlvswave) — uses these features to pit wave-compute vs ML
- [`../../AD2`](../../AD2) — the JS sensing backend this Go package mirrors
