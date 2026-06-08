// =============================================================================
// AD2/wavecompute — interferenceMvm: matrix-vector multiply by wave interference
// =============================================================================
// An optical/RF matrix-vector multiplier (an interferometer mesh or metasurface)
// computes y = M·x in ONE pass: the input vector is encoded as waves, and each
// output is the coherent SUM of those waves weighted by a matrix row — i.e.
// constructive/destructive interference. A real value is a phasor whose SIGN is
// a phase (0 for +, π for −), so subtraction is waves cancelling.
//
//   y_i = Σ_j M_ij · x_j     ← computed as the interference of the input waves
//
// This is the core of optical neural nets and quantum linear algebra's classical
// cousin. The DFT (wavefourier.js) is just ONE special matrix this mesh can be —
// the tests cross-check that mvm(dftMatrix, x) === dft(x). Pure, zero-dep.
// =============================================================================

/** Complex number helper. */
export const C = (re, im = 0) => ({ re, im });
const cadd = (a, b) => C(a.re + b.re, a.im + b.im);
const cmul = (a, b) => C(a.re * b.re - a.im * b.im, a.re * b.im + a.im * b.re);

/** A real value is a phasor: magnitude |v|, phase 0 (v≥0) or π (v<0). */
const toPhasor = (v) => (typeof v === 'number' ? C(v, 0) : v);

/**
 * Matrix-vector multiply by interference. `M` is rows of (numbers | {re,im});
 * `x` is (numbers | {re,im}). Each output is the coherent sum of the input
 * waves weighted by that row — the physics does all outputs simultaneously.
 * @returns {Array<{re:number, im:number}>}
 */
export function mvm(M, x) {
  const xs = x.map(toPhasor);
  return M.map((row) => {
    let acc = C(0, 0);
    for (let j = 0; j < row.length; j++) acc = cadd(acc, cmul(toPhasor(row[j]), xs[j]));
    return acc; // the interference sum at this output
  });
}

/** Output amplitudes |y_i| — what a detector reads (the measured power's root). */
export const amplitudes = (y) => y.map((c) => Math.hypot(c.re, c.im));

/** Real parts of a complex output (for real matrices/vectors). */
export const reals = (y) => y.map((c) => c.re);

/**
 * The DFT matrix — the configuration that makes this mesh perform a Fourier
 * transform. M[k][n] = e^{-2πi·kn/N}. Proof that the DFT is a special MVM.
 */
export function dftMatrix(N) {
  const M = [];
  for (let k = 0; k < N; k++) {
    const row = [];
    for (let n = 0; n < N; n++) {
      const a = (-2 * Math.PI * k * n) / N;
      row.push(C(Math.cos(a), Math.sin(a)));
    }
    M.push(row);
  }
  return M;
}
