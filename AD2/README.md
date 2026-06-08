# waves_worx/AD2 — the Analog Discovery 2 bench backend

A **real physical-layer backend** for the pure `waves_worx` codecs, built on the
[Digilent Analog Discovery 2](https://digilent.com/reference/test-and-measurement/analog-discovery-2/start)
(AD2). It's also the working **model of Pidar sensing** — how Ric turns raw
physical signal into grounded meaning — that you can hold in your hands.

> **Scope & intent.** For **authorized teaching, bench R&D, and consensual,
> co-present local transfer**, matching the parent library. Everything stays in
> the room: a tone goes out of a wire and comes back in. No cloud, no covert
> signalling.

---

## The wave-compute suite (start here)

This folder grew into a small, self-contained **wave-computing curriculum**: how
far classical waves reach toward the math quantum computers are famous for —
stated honestly, with the exponential walls named. Read it in this order:

| # | Module | What you learn |
|---|--------|----------------|
| 1 | `src/sense.js` · `src/dsp.js` *(this module)* | a signal → a feature map (Goertzel); the Pidar pipeline |
| 2 | [`quantum/`](quantum/) | quantum gates, Bell entanglement, Grover, QFT — as tone interference |
| 3 | [`wavecompute/`](wavecompute/) | wave-native kernels: Ising machine, wave Fourier transform, interference MVM, reservoir |
| 4 | [`../go/mlvswave`](../go/mlvswave) | *when* classical wave-compute beats ML — and when it can't |
| 5 | cross-repo [`eco/hdc`](../../hdc) | grounding symbols as hypervectors (the vector layer the features feed) |

**The capability map** — each kernel is the classical cousin of a quantum idea:

| Capability | Math for free | Quantum cousin | Built in |
|---|---|---|---|
| Oscillator Ising machine | MAX-CUT / QUBO optimization | quantum annealing | `wavecompute/ising.js` |
| Grover amplitude amplification | unstructured search | Grover | `quantum/` |
| Quantum gates as interference | superposition, entanglement | gate model | `quantum/` |
| Wave Fourier transform | a wave *is* a DFT | QFT | `wavecompute/wavefourier.js` |
| Interference MVM | dense matrix-vector multiply | quantum linear algebra | `wavecompute/interferenceMvm.js` |
| Reservoir computing | temporal pattern recognition | quantum reservoirs | `wavecompute/reservoir.js` |

> **The honest throughline:** each kernel gets a real speedup by letting *physics
> do the math* (interference, resonance, relaxation), paid for in analog precision
> + I/O, and restricted to *specific* problem classes. That restriction is exactly
> why it's quantum-*like*, not quantum.

---

## The model: AD2 sensing ↔ Pidar ↔ how Ric processes

Pidar is "robot Ric, checked-in *here-now*, turning what it **senses** into a
**feature map** it can ground and remember." The AD2 makes that pipeline
physical and debuggable — it converts continuous reality into discrete samples,
and we process those into symbols → bytes → meaning. That's the *same shape* as
the EPU/TST grounding pipeline, just at the signal layer:

```
        PHYSICAL WORLD            DISCRETE                 GROUNDED
        (continuous V)            (samples)                (meaning)

  AWG ──tone──▶ wire ──▶ │ AD2 ADC │ ──▶ samples ──▶ Goertzel ──▶ #symbol ──▶ bytes
        W1            1+   100 MS/s        Float64       (dsp.js)    (0..15)    (modem.decode)
                                              │
                                              └──▶ senseFeatures ──▶ { peakSymbol, centroidHz, bandEnergies[16] }
                                                     (sense.js)         the Pidar FEATURE MAP

  ── and the exact parallel in how Ric/EPU processes everything else ──

  raw signal   ──▶  sample/quantize  ──▶  classical #symbol  ──▶  feature map  ──▶  (later) vectors
  (text, sound,     (geohash, time,       (#emoji / type,         (TST grounding   (deferred — the
   photo, RF)        tone window)          peakSymbol)             tuple)            narrowed set)
```

The point both halves share (and the reason this isn't a toy): **ground in cheap,
classical, debuggable features first.** TST grounds in time+space+`#type` *before*
running any vector search; `senseFeatures` produces `peakSymbol` + `centroidHz` +
`bandEnergies` *before* any embedding. The classical key is also what debugs the
vectors layered above it — same philosophy as `src/domain/vibe/unicodeType.js`.

| Pidar / EPU concept | AD2 realization (this module) |
| --- | --- |
| sense the here-now | scope captures real voltage samples |
| the `#symbol` / `#emoji` token (the "what") | `peakSymbol` — strongest of 16 FSK tones |
| a 1-D "vibe" scalar | `centroidHz` — spectral center of mass |
| the pre-vector feature map | `bandEnergies[16]` — energy per tone |
| ground, then narrow, then (maybe) vectorize | classical Goertzel first; embeddings stay deferred |

---

## Architecture (pure core, thin adapter)

Identical discipline to the parent lib — the DSP is pure and node-testable; the
hardware is a thin adapter you can swap for a mock.

```
  bench.mjs            run the whole loop: message ⇄ tones ⇄ device ⇄ meaning
  ─────────────────────────────────────────────────────────────────────────
  sense.js   (pure)    captured samples → Pidar feature map
  dsp.js     (pure)    samples → tones  (Goertzel single-bin detection)
  device.js  (pure)    the AD2 contract + createMockDevice() (synthesizes samples)
  ─────────────────────────────────────────────────────────────────────────
  python/ad2_driver.py REAL hardware I/O only — AWG plays tones, scope captures
```

`device.js` defines the entire physical hop as one method:

```
AD2_DEVICE := { kind, sampleRate, playAndCapture(tones) → samples, close() }
```

`createMockDevice()` implements it in pure JS (a clean W1→1+ loopback, optional
noise) so the full sense→decode loop runs with **no hardware attached** — the
sibling of `waves_worx`'s `createLoopbackChannel`.

---

## Run it now (no hardware)

```bash
node --test test/*.test.mjs          # pure DSP + loop round-trip + sense
node bench.mjs --mock "meet at 9"    # synthesize the whole loop end-to-end
node bench.mjs --mock --noise 0.3 hi # model a dirtier analog channel
```

## Run it on a real AD2 (Fedora)

1. Install **WaveForms** (the official Fedora RPM ships `libdwf`).
2. Wire a loopback on the AD2 flywires: **`W1` → `1+`**, and **`⏚` → `1-`**.
3. Close the WaveForms GUI (it locks the device), then:

```bash
python3 python/ad2_driver.py --probe     # confirm the SDK opens the device
node bench.mjs "meet at 9"               # play over the wire, capture, decode
```

The JS owns all modulation/recovery; `ad2_driver.py` is the dumb I/O pipe (tones
in over stdin → captured samples out over stdout). Swap the wire loopback for two
breadboarded RX/TX nodes and you're transmitting over a real analog medium.

### Why audio-band FSK fits the AD2 comfortably

The `modem` tones live at **1200–2400 Hz**. The AD2 scope samples at up to
**100 MS/s** across a **30 MHz** bandwidth — four-plus orders of magnitude of
headroom over Nyquist for these tones. We capture at a modest 48 kHz (still ~20×
the top tone), so windows stay small (< the 16384-sample buffer) and Goertzel is
cheap. The AD2's real ceiling matters when you move the carrier up toward RF/IF —
that's the upgrade path, not a limit here.

---

## Voltage, grounding & the Wheatstone-bridge front-end

When you wire a *real* sensor in (instead of the `W1 → 1+` loopback), the analog
details matter — both for not damaging the device and for reading small signals
cleanly.

### AD2 voltage limits — do not exceed

| Pin | Limit | Role |
|---|---|---|
| Scope `1±` / `2±` | **±25 V** single-ended, **±50 V** differential (ESD clamp ~±50 V) | measurement inputs · 1 MΩ ∥ 24 pF |
| AWG `W1` / `W2` | **±5 V** | signal source |
| Power `V+` / `V−` | **+0.5…+5 V** / **−0.5…−5 V** · 500 mW total on USB (≈2.1 W with the 5 V/2.5 A aux brick) | excite the sensor / bridge |
| Digital `0–15` | 3.3 V CMOS out; inputs **5 V-tolerant** — never feed >5 V | logic / triggers |

Rules of thumb: never connect mains or anything you haven't bounded under these
limits — the clamps protect against accidents, they are **not** a design margin.
Tie the sensor ground to an AD2 **`⏚`** flywire so everything shares one
reference, and keep that ground path short to avoid loop noise.

### Use the DIFFERENTIAL inputs for a voltage *difference*

The scope channels are **true differential** (`1+` and `1−`), not single-ended-
with-a-ground. That is exactly what you want for a sensor whose signal is a small
**difference** riding on a larger common-mode level: measure `1+ − 1−` directly
and the common-mode cancels.

### The Wheatstone bridge — a tiny ΔR → a readable Δ-voltage

Many "wave-like" sensors (strain gauges, thermistors, photo/piezo/magneto-
resistive elements) transduce a physical wave — strain, heat, light, field —
into a *tiny resistance change*. A **Wheatstone bridge** converts that ΔR into a
small **differential voltage** the AD2 reads natively:

```
                 V_exc  (AD2 V+ or a stable reference)
               ┌────┴────┐
              R1        R3
               │         │
       1+ ─────A         B───── 1−      AD2 differential scope reads V_out = A − B
               │         │
              R2        R4
               └────┬────┘
                  GND (⏚)
```

Balanced (`R1/R2 = R3/R4`) → `V_out = 0`. A small change in one arm gives, for a
quarter-bridge (one active element, near-equal nominal R):

```
  V_out ≈ V_exc · (ΔR / R) / 4        (small-signal)
```

So the bridge parks the sensor's resting value at **zero volts** and outputs only
the *change* — letting the AD2's 14-bit ADC (~0.3 mV on a small range) resolve a
parts-per-thousand ΔR that would vanish in a single-ended reading.

Practical notes:
- **Excitation:** the AD2 `V+` works, but it is not a precision reference — for
  accuracy, also sample `V_exc` on scope channel 2 and use the **ratio**
  `V_out / V_exc` (ratiometric → cancels excitation drift).
- **Gain:** bridge outputs are millivolts; an **instrumentation amplifier** (INA)
  between `V_out` and `1±` uses more of the ADC's dynamic range. Without one, drop
  the AD2 to a smaller input range for finer resolution.
- **Drift cancellation:** a *half-bridge* (two active arms, opposite legs) doubles
  sensitivity and cancels temperature drift; a *full-bridge* (four active) is best
  on both. A dummy element in an adjacent arm gives passive temp compensation.

### Why this matters for the wave-compute pipeline

The bridge output **is** the sample stream `dsp.js` / `sense.js` consume: a small
analog voltage that varies over time with the sensed physical wave. The full chain
is *physical wave → sensor ΔR → bridge Δ-voltage → AD2 differential ADC →
Goertzel / `senseFeatures` → the wave-compute kernels*. The Wheatstone bridge is
the honest analog front-end that lets a real-world signal enter everything above.

---

## School / capstone hooks

- **Measured channel characterization** — sweep `--noise` (mock) or add real
  attenuation/cable length (hardware) and chart decode success vs SNR; the
  `sense` RMS + band energies give you the measurement axes for free.
- **Pidar sensing demo** — feed non-data signals (a function generator, a
  speaker, ambient tone) into `1+` and watch `senseFeatures` classify the
  dominant symbol + centroid live: "the room has a vibe, here it is as a vector."
- **Real transport** — implement a second `AD2_DEVICE` using AnalogOut *play* +
  AnalogIn *record* (streaming) for continuous, non-per-tone transfer.
- **Cross-medium** — point the same codec at light (LED → photodiode into `1+`)
  or RF (downconvert to IF first); only the adapter changes.

## Go ML / EPU bridge

The sibling Go module now includes `go/ad2ml`, a stdlib-only mirror of the AD2
Pidar feature map:

```go
import "github.com/ericdequ/waves_worx/go/ad2ml"

features := ad2ml.SenseFeatures(samples, 48000, ad2ml.DefaultConfig())
vector := features.FeatureVector32() // [rms, peakSymbol, centroidHz, bands...]
```

Use `ad2ml` as the stable middle layer before adding heavier adapters:

- **Gonum**: FFT/windowing/filter experiments over real captures.
- **GoMLX**: pack `FeatureVector32` into tensors for local inference.
- **cgo/purego WaveForms**: a future Go hardware adapter replacing the Python
  `libdwf` bridge while keeping this feature API unchanged.
- **EPU/WAP**: convert physical signal features into `.ric` manifests, EPU
  prompts, or a wave-analog-processor control loop.

## Test

```bash
node --test test/*.test.mjs
env GOCACHE=/tmp/go-build-cache go -C ../go test ./ad2ml
```

Pure, hardware-free: Goertzel tone detection, the full encode→capture→decode
round-trip (clean + noisy), and `senseFeatures` grounding a known tone.

## References

- G. Goertzel, "An algorithm for the evaluation of finite trigonometric series,"
  *Amer. Math. Monthly* 65 (1958) — the single-bin DFT behind `dsp.js`.
- P. Kanerva, "Hyperdimensional Computing," *Cognitive Computation* 1 (2009) —
  the grounding layer the feature map feeds (see [`eco/hdc`](../../hdc)).

## See also

- [`quantum/`](quantum/) — quantum algorithms as wave interference
- [`wavecompute/`](wavecompute/) — the wave-native compute kernels
- [`../go/ad2ml`](../go/ad2ml) — the Go mirror of `sense.js`
- [`eco/hdc`](../../hdc) — symbols → hypervectors (features → vectors)
