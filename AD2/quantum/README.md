# AD2/quantum — quantum algorithms as wave interference

A standalone (non-BEV) **state-vector emulator framed as waves**: each of the 2ⁿ
quantum amplitudes *is* a wave mode — a tone with a magnitude and a phase. Gates
are unitary mixing, so running a circuit is the constructive/destructive
**interference** of those phased tones; "measurement" is **wave power** (|amp|²),
which is what an AD2 reading the output channels would see.

```
  qubit amplitude  ↔  tone (magnitude, phase)
  quantum gate      ↔  interference / linear mixing of tones
  measurement       ↔  wave power per mode (|amp|²)
```

```bash
node --test quantum.test.mjs    # X, Hadamard, Bell entanglement, Grover, unitarity
node demo.mjs                   # Grover search, printed as wave modes
```

## What it honestly is (and isn't)

- ✅ A faithful sandbox for **small circuits** (3–5 qubits): superposition,
  entanglement, Grover, QFT — real quantum *math*, no hardware, room temperature,
  no decoherence.
- ✅ A bridge to the AD2: `waveModes()` renders the state as tones in the modem
  band, so a state can literally be *played and captured*.
- ❌ **Not** a quantum computer. It's classical, and n qubits cost **2ⁿ wave
  modes** — the exponential wall. Entanglement here is correlated wave paths, not
  the real-resource entanglement that gives quantum its scaling.

## API (`QState`)

| Method | Returns | Does |
|---|---|---|
| `new QState(n)` | — | n-qubit register, starts in \|0…0⟩ |
| `.h .x .y .z .s .t(q)` · `.phase(q,θ)` | `this` | single-qubit gates (wave mixing) |
| `.cnot(c,t)` · `.cphase(c,t,θ)` · `.swap(a,b)` | `this` | two-qubit gates |
| `.oracle(pred)` · `.diffuse()` | `this` | Grover phase-flip + inversion-about-mean |
| `.qft()` | `this` | Quantum Fourier Transform (verified **==** the wave DFT) |
| `.probs()` · `.sample(rng)` | `Float64Array` · `int` | measurement: power per mode / a collapse |
| `.waveModes(opts)` | `[{basis,freqHz,amplitude,phaseRad,power}]` | render the state as AD2 tones |

Headline: Grover in 2 qubits reaches the marked state with **certainty in one
iteration** (see the test) — and `.qft()` is provably the same operation as the
classical wave Fourier transform next door.

## References

- M. A. Nielsen & I. L. Chuang, *Quantum Computation and Quantum Information*
  (Cambridge, 2000) — gates, the QFT, measurement.
- L. K. Grover, "A fast quantum mechanical algorithm for database search,"
  *STOC* (1996).

## See also

- [`../wavecompute/wavefourier.js`](../wavecompute) — the classical wave Fourier
  transform, **proven equal** to `.qft()`
- [`../README.md`](../README.md) — the wave-compute suite gateway
- [`../src/dsp.js`](../src) — Goertzel: a single bin of that same transform
