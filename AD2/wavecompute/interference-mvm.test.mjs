// =============================================================================
// AD2/wavecompute — interference MVM tests. Matrix-vector multiply by wave
// interference, the destructive/constructive essence, and the proof that the
// DFT is just one special configuration of the same mesh.
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { dft } from './wavefourier.js';
import { amplitudes, dftMatrix, mvm, reals } from './interferenceMvm.js';

const close = (a, b, eps = 1e-9) => Math.abs(a - b) < eps;

test('a real matrix-vector product comes out of the interference sum', () => {
  const M = [[1, 2], [3, 4]];
  const y = reals(mvm(M, [5, 6]));
  assert.ok(close(y[0], 17) && close(y[1], 39));
});

test('destructive interference: equal-and-opposite weights cancel to zero', () => {
  // a row [1, -1] on input [3, 3] — the two waves are π out of phase → null.
  assert.ok(close(amplitudes(mvm([[1, -1]], [3, 3]))[0], 0));
  // constructive: aligned weights add.
  assert.ok(close(reals(mvm([[1, 1]], [3, 3]))[0], 6));
});

test('a negative value is carried as a π-phase wave (sign = phase)', () => {
  assert.ok(close(reals(mvm([[1]], [-2]))[0], -2));
});

test('the identity matrix passes the vector through unchanged', () => {
  const I = [[1, 0, 0], [0, 1, 0], [0, 0, 1]];
  assert.deepEqual(reals(mvm(I, [7, -3, 2])).map((v) => Math.round(v)), [7, -3, 2]);
});

test('the DFT is ONE special MVM: mvm(dftMatrix, x) === dft(x)', () => {
  const x = [1, -2, 3, 0.5, -1, 4, 2, -3];
  const viaMesh = mvm(dftMatrix(x.length), x);
  const viaWave = dft(Float64Array.from(x));
  for (let k = 0; k < x.length; k++) {
    assert.ok(close(viaMesh[k].re, viaWave.re[k]) && close(viaMesh[k].im, viaWave.im[k]));
  }
});
