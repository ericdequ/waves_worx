#!/usr/bin/env node
// =============================================================================
// AD2/wavecompute — wave Fourier transform demo.   node fourier-demo.mjs
// =============================================================================
import { binFreqs, convolve, dft, magnitudes } from './wavefourier.js';

const N = 16;
const bar = (v, max) => '█'.repeat(Math.round((v / max) * 24));

// 1) Encode a "vector" as a wave: three tones at bins 1, 4, 6 with given strengths.
const freqs = binFreqs(N);
const wanted = { 1: 1.0, 4: 0.6, 6: 0.3 };
const wave = Float64Array.from({ length: N }, (_, n) => {
  let v = 0;
  for (const [k, amp] of Object.entries(wanted)) v += amp * Math.cos((2 * Math.PI * k * n) / N);
  return v;
});

console.log('1) a wave carrying three tones — its spectrum reads them straight back:\n');
const mag = magnitudes(...Object.values(dft(wave)));
const top = Math.max(...mag);
for (let k = 0; k <= N / 2; k++) {
  if (mag[k] > 1e-6) console.log(`   bin ${String(k).padStart(2)} (${freqs[k]}Hz)  ${bar(mag[k], top)}`);
}

// 2) The speedup: filtering = pointwise multiply in the wave domain.
console.log('\n2) convolution theorem — smoothing a spiky signal by frequency multiply:\n');
const signal = [0, 0, 8, 0, 0, 0, 4, 0];
const kernel = [1 / 3, 1 / 3, 0, 0, 0, 0, 0, 1 / 3]; // 3-tap circular moving average
const smoothed = convolve(signal, kernel);
console.log(`   signal   : [${signal.join(', ')}]`);
console.log(`   smoothed : [${[...smoothed].map((v) => v.toFixed(2)).join(', ')}]`);
console.log('   (an O(N²) convolution done as O(N) multiplies — that is the Fourier speedup)\n');

console.log('→ same transform a quantum computer runs as the QFT (AD2/quantum qft()),');
console.log('  here executed by wave interference. Verified equal in wavefourier.test.mjs.\n');
