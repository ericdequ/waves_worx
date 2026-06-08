// =============================================================================
// AD2/wavecompute — wavefourier: a propagating wave IS a Fourier transform
// =============================================================================
// The Discrete Fourier Transform projects a wave onto every frequency at once —
// and that is exactly what physical waves do: a signal's spectrum is its Fourier
// transform, computed by the medium as the wave propagates (a lens does a 2D FT
// in O(1) physical time). Here the DFT is written as "project onto each bin",
// which is literally the AD2's Goertzel (../src/dsp.js) run across the whole
// band instead of a single tone.
//
//   analysis  : wave (time)      → spectrum (frequency)   = dft
//   synthesis : spectrum         → wave (time)            = idft
//   the speedup: CONVOLUTION in time = pointwise MULTIPLY in frequency.
//                an O(N²) convolution becomes O(N) multiplies in the wave domain.
//
// The quantum cousin is the QFT (../quantum/qwave.js qft()) — the same transform
// executed by interference; the two are cross-checked in the tests. Pure, zero-dep.
// =============================================================================

/**
 * Discrete Fourier Transform. Each output bin k is the wave's projection onto
 * frequency k — a single Goertzel, run for every bin.
 * @param {ArrayLike<number>} re  real part of the signal
 * @param {ArrayLike<number>|null} [im]  imaginary part (defaults to zeros)
 * @returns {{ re: Float64Array, im: Float64Array }}
 */
export function dft(re, im = null) {
  const N = re.length;
  const outRe = new Float64Array(N);
  const outIm = new Float64Array(N);
  for (let k = 0; k < N; k++) {
    let sRe = 0;
    let sIm = 0;
    for (let n = 0; n < N; n++) {
      const ang = (-2 * Math.PI * k * n) / N;
      const c = Math.cos(ang);
      const s = Math.sin(ang);
      const xr = re[n];
      const xi = im ? im[n] : 0;
      sRe += xr * c - xi * s;
      sIm += xr * s + xi * c;
    }
    outRe[k] = sRe;
    outIm[k] = sIm;
  }
  return { re: outRe, im: outIm };
}

/** Inverse DFT: rebuild the time-domain wave from its spectrum. */
export function idft(re, im) {
  const N = re.length;
  const outRe = new Float64Array(N);
  const outIm = new Float64Array(N);
  for (let n = 0; n < N; n++) {
    let sRe = 0;
    let sIm = 0;
    for (let k = 0; k < N; k++) {
      const ang = (2 * Math.PI * k * n) / N;
      const c = Math.cos(ang);
      const s = Math.sin(ang);
      sRe += re[k] * c - im[k] * s;
      sIm += re[k] * s + im[k] * c;
    }
    outRe[n] = sRe / N;
    outIm[n] = sIm / N;
  }
  return { re: outRe, im: outIm };
}

/** Per-bin magnitude (the spectrum power profile). */
export function magnitudes(re, im) {
  const out = new Float64Array(re.length);
  for (let i = 0; i < re.length; i++) out[i] = Math.hypot(re[i], im[i]);
  return out;
}

/** Per-bin phase (radians). */
export function phases(re, im) {
  const out = new Float64Array(re.length);
  for (let i = 0; i < re.length; i++) out[i] = Math.atan2(im[i], re[i]);
  return out;
}

/**
 * Circular convolution via the wave domain — the headline capability. Transform
 * both signals, multiply pointwise (waves interfering), inverse-transform. This
 * is where the Fourier "speedup" lives: convolution becomes multiplication.
 * @returns {Float64Array} the real convolution result
 */
export function convolve(a, b) {
  const N = a.length;
  const A = dft(Float64Array.from(a));
  const B = dft(Float64Array.from(b));
  const pRe = new Float64Array(N);
  const pIm = new Float64Array(N);
  for (let k = 0; k < N; k++) {
    pRe[k] = A.re[k] * B.re[k] - A.im[k] * B.im[k];
    pIm[k] = A.re[k] * B.im[k] + A.im[k] * B.re[k];
  }
  return idft(pRe, pIm).re;
}

/** Map DFT bins to Hz in the AD2 modem band, so a spectrum reads as tones. */
export function binFreqs(N, { baseFreq = 1200, stepFreq = 80 } = {}) {
  return Array.from({ length: N }, (_, k) => baseFreq + k * stepFreq);
}
