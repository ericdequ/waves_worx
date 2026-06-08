#!/usr/bin/env node
// =============================================================================
// AD2/wavecompute — interference MVM demo.   node mvm-demo.mjs
// =============================================================================
import { amplitudes, dftMatrix, mvm, reals } from './interferenceMvm.js';
import { dft, magnitudes } from './wavefourier.js';

console.log('1) a 3×3 matrix-vector product as one wave pass (all outputs at once):\n');
const M = [
  [2, 0, 1],
  [1, 3, 0],
  [0, 1, 2],
];
const x = [1, 2, 3];
console.log(`   M·[${x.join(', ')}] = [${reals(mvm(M, x)).map((v) => v.toFixed(0)).join(', ')}]`);

console.log('\n2) interference: equal-and-opposite weights cancel, aligned weights add:\n');
console.log(`   [1, -1]·[5, 5] → |y| = ${amplitudes(mvm([[1, -1]], [5, 5]))[0].toFixed(2)}  (destructive → null)`);
console.log(`   [1,  1]·[5, 5] →  y  = ${reals(mvm([[1, 1]], [5, 5]))[0].toFixed(2)}  (constructive → sum)`);

console.log('\n3) configure the SAME mesh as the DFT matrix → it Fourier-transforms:\n');
const sig = [1, -2, 3, 0.5, -1, 4, 2, -3];
const viaMesh = magnitudes(...transpose(mvm(dftMatrix(sig.length), sig)));
const viaWave = magnitudes(...Object.values(dft(Float64Array.from(sig))));
console.log(`   mesh spectrum : [${[...viaMesh].map((v) => v.toFixed(2)).join(', ')}]`);
console.log(`   wave spectrum : [${[...viaWave].map((v) => v.toFixed(2)).join(', ')}]   (identical)\n`);

// helper: split {re,im}[] into (reArr, imArr) for magnitudes()
function transpose(y) {
  return [y.map((c) => c.re), y.map((c) => c.im)];
}
