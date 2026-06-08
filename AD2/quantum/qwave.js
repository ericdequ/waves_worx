// =============================================================================
// AD2/quantum — qwave: quantum algorithms as wave interference (an emulator)
// =============================================================================
// A classical state-vector simulator framed as WAVES. An n-qubit state is 2^n
// complex amplitudes; here each amplitude IS a wave mode — a tone with a
// magnitude and a phase. Quantum gates are unitary matrices, and applying them
// is exactly the constructive/destructive interference of those phased waves.
// "Measurement" is wave power: |amplitude|^2, the fraction of energy in each
// mode (what an AD2 reading the output channels would see).
//
//   qubit amplitude  ↔  tone (magnitude, phase)
//   quantum gate      ↔  interference / linear mixing of tones
//   measurement       ↔  wave power per mode  (|amp|^2)
//
// NOT a quantum computer (it's classical, and n qubits cost 2^n wave modes — the
// exponential wall). It's a faithful sandbox for SMALL circuits: Hadamard,
// entanglement (Bell), Grover, QFT on a few qubits. Standalone, pure, zero-dep.
// =============================================================================

const c = (re, im = 0) => ({ re, im });
const INV_SQRT2 = Math.SQRT1_2;

/** Standard single-qubit gates as 2×2 complex matrices {m00,m01,m10,m11}. */
export const GATES = Object.freeze({
  H: { m00: c(INV_SQRT2), m01: c(INV_SQRT2), m10: c(INV_SQRT2), m11: c(-INV_SQRT2) },
  X: { m00: c(0), m01: c(1), m10: c(1), m11: c(0) },
  Y: { m00: c(0), m01: c(0, -1), m10: c(0, 1), m11: c(0) },
  Z: { m00: c(1), m01: c(0), m10: c(0), m11: c(-1) },
  S: { m00: c(1), m01: c(0), m10: c(0), m11: c(0, 1) },
  T: { m00: c(1), m01: c(0), m10: c(0), m11: c(Math.cos(Math.PI / 4), Math.sin(Math.PI / 4)) },
});

/** A phase gate by angle θ: diag(1, e^{iθ}). */
export const phaseGate = (theta) => ({
  m00: c(1),
  m01: c(0),
  m10: c(0),
  m11: c(Math.cos(theta), Math.sin(theta)),
});

/**
 * An n-qubit register. Qubit 0 is the least-significant bit of the basis index.
 * State starts in |0…0⟩. Methods mutate and return `this` for chaining.
 */
export class QState {
  constructor(n) {
    this.n = n;
    const size = 1 << n;
    this.re = new Float64Array(size);
    this.im = new Float64Array(size);
    this.re[0] = 1; // |0…0⟩
  }

  /** Apply a single-qubit gate to qubit q — the interference of its two modes. */
  apply(q, g) {
    const step = 1 << q;
    for (let i = 0; i < this.re.length; i++) {
      if ((i & step) === 0) {
        const j = i | step;
        const aRe = this.re[i];
        const aIm = this.im[i];
        const bRe = this.re[j];
        const bIm = this.im[j];
        this.re[i] = g.m00.re * aRe - g.m00.im * aIm + g.m01.re * bRe - g.m01.im * bIm;
        this.im[i] = g.m00.re * aIm + g.m00.im * aRe + g.m01.re * bIm + g.m01.im * bRe;
        this.re[j] = g.m10.re * aRe - g.m10.im * aIm + g.m11.re * bRe - g.m11.im * bIm;
        this.im[j] = g.m10.re * aIm + g.m10.im * aRe + g.m11.re * bIm + g.m11.im * bRe;
      }
    }
    return this;
  }

  h(q) { return this.apply(q, GATES.H); }
  x(q) { return this.apply(q, GATES.X); }
  y(q) { return this.apply(q, GATES.Y); }
  z(q) { return this.apply(q, GATES.Z); }
  s(q) { return this.apply(q, GATES.S); }
  t(q) { return this.apply(q, GATES.T); }
  phase(q, theta) { return this.apply(q, phaseGate(theta)); }

  /** Controlled-phase: multiply the |…1…1…⟩ modes (both bits set) by e^{iθ}. */
  cphase(control, target, theta) {
    const cBit = 1 << control;
    const tBit = 1 << target;
    const cos = Math.cos(theta);
    const sin = Math.sin(theta);
    for (let i = 0; i < this.re.length; i++) {
      if ((i & cBit) !== 0 && (i & tBit) !== 0) {
        const r = this.re[i];
        const im = this.im[i];
        this.re[i] = r * cos - im * sin;
        this.im[i] = r * sin + im * cos;
      }
    }
    return this;
  }

  /** Swap two qubits (exchange their amplitudes). */
  swap(a, b) {
    const aBit = 1 << a;
    const bBit = 1 << b;
    for (let i = 0; i < this.re.length; i++) {
      if ((i & aBit) !== 0 && (i & bBit) === 0) {
        const j = (i & ~aBit) | bBit;
        const r = this.re[i]; this.re[i] = this.re[j]; this.re[j] = r;
        const im = this.im[i]; this.im[i] = this.im[j]; this.im[j] = im;
      }
    }
    return this;
  }

  /**
   * Quantum Fourier Transform — H + controlled phases + bit-reversal. On a basis
   * state |x⟩ it produces (1/√N) Σ e^{2πi·xy/N} |y⟩: the DFT, executed by
   * interference. This is the quantum cousin of AD2/wavecompute's wave Fourier
   * transform — the two are verified equal in the wavecompute tests.
   */
  qft() {
    const n = this.n;
    for (let j = n - 1; j >= 0; j--) {
      this.h(j);
      for (let k = j - 1; k >= 0; k--) this.cphase(k, j, Math.PI / (1 << (j - k)));
    }
    for (let i = 0; i < n >> 1; i++) this.swap(i, n - 1 - i);
    return this;
  }

  /** Controlled-NOT: flip `target`'s bit in every mode where `control` is set. */
  cnot(control, target) {
    const cBit = 1 << control;
    const tBit = 1 << target;
    for (let i = 0; i < this.re.length; i++) {
      if ((i & cBit) !== 0 && (i & tBit) === 0) {
        const j = i | tBit;
        const tr = this.re[i]; this.re[i] = this.re[j]; this.re[j] = tr;
        const ti = this.im[i]; this.im[i] = this.im[j]; this.im[j] = ti;
      }
    }
    return this;
  }

  /** Apply H to every qubit (the uniform-superposition / diffusion building block). */
  hAll() {
    for (let q = 0; q < this.n; q++) this.h(q);
    return this;
  }

  /** Phase oracle: negate the amplitude of every basis index where pred(i) is true. */
  oracle(pred) {
    for (let i = 0; i < this.re.length; i++) {
      if (pred(i)) { this.re[i] = -this.re[i]; this.im[i] = -this.im[i]; }
    }
    return this;
  }

  /** Grover diffusion (inversion about the mean) = H⊗ⁿ · (negate all but |0⟩) · H⊗ⁿ. */
  diffuse() {
    return this.hAll().oracle((i) => i !== 0).hAll();
  }

  /** Measurement probabilities — wave power per mode, |amp|². Sums to 1. */
  probs() {
    const p = new Float64Array(this.re.length);
    for (let i = 0; i < p.length; i++) p[i] = this.re[i] * this.re[i] + this.im[i] * this.im[i];
    return p;
  }

  /**
   * Render the state AS WAVES: one tone per basis mode (magnitude + phase). This
   * is the literal bridge — the amplitude vector is a set of phased tones whose
   * interference is the linear algebra. Defaults match the AD2 modem band.
   */
  waveModes({ baseFreq = 1200, stepFreq = 80 } = {}) {
    const out = [];
    for (let i = 0; i < this.re.length; i++) {
      const amplitude = Math.hypot(this.re[i], this.im[i]);
      if (amplitude > 1e-9) {
        out.push({
          basis: i,
          bits: i.toString(2).padStart(this.n, '0'),
          freqHz: baseFreq + i * stepFreq,
          amplitude,
          phaseRad: Math.atan2(this.im[i], this.re[i]),
          power: amplitude * amplitude,
        });
      }
    }
    return out;
  }

  /** Collapse to a basis index using a [0,1) random draw (measurement). */
  sample(rng) {
    const r = rng();
    const p = this.probs();
    let acc = 0;
    for (let i = 0; i < p.length; i++) {
      acc += p[i];
      if (r < acc) return i;
    }
    return p.length - 1;
  }
}
