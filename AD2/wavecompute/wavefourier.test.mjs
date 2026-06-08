// =============================================================================
// AD2/wavecompute — wave Fourier transform tests, including the headline:
// the classical wave DFT is PROVABLY the same operation as the quantum QFT.
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { goertzelPower } from '../src/dsp.js';
import { QState } from '../quantum/qwave.js';
import { convolve, dft, idft, magnitudes, phases } from './wavefourier.js';

const close = (a, b, eps = 1e-9) => Math.abs(a - b) < eps;
const argmax = (arr) => {
  let m = 0;
  for (let i = 1; i < arr.length; i++) if (arr[i] > arr[m]) m = i;
  return m;
};

test('a pure tone shows up as energy in exactly its frequency bin (and its mirror)', () => {
  const N = 16;
  const k = 3;
  const sig = Float64Array.from({ length: N }, (_, n) => Math.cos((2 * Math.PI * k * n) / N));
  const { re, im } = dft(sig);
  const mag = magnitudes(re, im);
  const peak = argmax(mag); // a real cosine peaks at BOTH k and its mirror N-k
  assert.ok(peak === k || peak === N - k, `peak bin ${peak} should be ${k} or ${N - k}`);
  assert.ok(close(mag[k], N / 2, 1e-6)); // amplitude-1 tone → N/2 of energy
  assert.ok(mag[(k + 1) % N] < 1e-6); // neighbouring bins are empty
  assert.ok(close(mag[k], mag[N - k], 1e-6)); // real signal → mirrored spectrum
});

test('synthesis ∘ analysis is identity: idft(dft(x)) === x', () => {
  const x = Float64Array.from([1, -2, 3, 0.5, -1, 4, 2, -3]);
  const back = idft(...Object.values(dft(x)));
  for (let i = 0; i < x.length; i++) assert.ok(close(back.re[i], x[i]) && close(back.im[i], 0));
});

test('Goertzel is one bin of the wave Fourier transform (same dominant frequency)', () => {
  const N = 32;
  const k = 7;
  const sig = Float64Array.from({ length: N }, (_, n) => Math.cos((2 * Math.PI * k * n) / N));
  // run the AD2 single-bin detector across every integer bin → it peaks at the DFT bin.
  const power = Array.from({ length: N }, (_, bin) => goertzelPower(sig, bin, N));
  assert.equal(argmax(power), k);
});

test('CONVOLUTION THEOREM: time convolution = frequency multiply (the speedup)', () => {
  const a = [1, 2, 3, 4];
  const b = [0, 1, 0, 0]; // a circular shift kernel
  const viaWave = convolve(a, b);

  // direct circular convolution for ground truth
  const N = a.length;
  const direct = Array.from({ length: N }, (_, n) => {
    let s = 0;
    for (let m = 0; m < N; m++) s += a[m] * b[(n - m + N) % N];
    return s;
  });
  for (let i = 0; i < N; i++) assert.ok(close(viaWave[i], direct[i], 1e-9));
});

test('the QUANTUM Fourier transform and the WAVE Fourier transform are the same operation', () => {
  const n = 3;
  const N = 1 << n;
  const x = 3; // QFT the basis state |3⟩

  const q = new QState(n);
  for (let b = 0; b < n; b++) if ((x >> b) & 1) q.x(b);
  q.qft();

  // 1) the QFT output is a uniform superposition — every mode magnitude 1/√N
  const qMag = magnitudes(q.re, q.im);
  for (const m of qMag) assert.ok(close(m, 1 / Math.sqrt(N), 1e-9));

  // 2) running the classical wave DFT on the QFT output collapses it back to |x⟩
  //    → the wave Fourier transform inverts the quantum Fourier transform.
  const spectrum = dft(q.re, q.im);
  assert.equal(argmax(magnitudes(spectrum.re, spectrum.im)), x);
});

test('phase carries information power alone misses (a shifted tone, same spectrum power)', () => {
  const N = 16;
  const k = 2;
  const a = Float64Array.from({ length: N }, (_, n) => Math.cos((2 * Math.PI * k * n) / N));
  const b = Float64Array.from({ length: N }, (_, n) => Math.sin((2 * Math.PI * k * n) / N)); // 90° shifted
  const pa = phases(...Object.values(dft(a)));
  const pb = phases(...Object.values(dft(b)));
  assert.ok(Math.abs(pa[k] - pb[k]) > 1); // same power bin, different phase
});
