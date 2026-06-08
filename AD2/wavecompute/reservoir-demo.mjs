#!/usr/bin/env node
// =============================================================================
// AD2/wavecompute — reservoir computing demo.   node reservoir-demo.mjs
// =============================================================================
import { createReservoir, dot, ridgeFit, runStream, stateFeature } from './reservoir.js';

const lcg = (seed) => { let s = seed >>> 0 || 1; return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32; };

// Temporal task: y(t) = u(t-1) XOR u(t-2) — needs memory AND nonlinearity.
const T = 1200;
const rnd = lcg(42);
const u = Array.from({ length: T }, () => (rnd() < 0.5 ? 0 : 1));
const y = u.map((_, t) => (t >= 2 ? ((u[t - 1] ^ u[t - 2]) ? 1 : -1) : 0));

function accuracy(features) {
  const idx = [];
  for (let t = 60; t < T; t++) idx.push(t);
  const X = idx.map((t) => features[t]);
  const Y = idx.map((t) => y[t]);
  const split = Math.floor(idx.length * 0.7);
  const w = ridgeFit(X.slice(0, split), Y.slice(0, split), 1e-3);
  let c = 0;
  for (let i = split; i < idx.length; i++) if ((dot(w, X[i]) >= 0 ? 1 : -1) === Y[i]) c++;
  return c / (idx.length - split);
}

console.log('task: predict  y(t) = u(t-1) XOR u(t-2)  from a random bit stream');
console.log('      (needs 2-step MEMORY and a NONLINEAR boundary — the classic reservoir test)\n');

const res = createReservoir({ nIn: 1, nRes: 80, spectralRadius: 0.92, inputScale: 1, seed: 7 });
const reservoirAcc = accuracy(runStream(res, u.map((v) => [v])).map(stateFeature));

const rawFeatures = u.map((_, t) => [1, t >= 1 ? u[t - 1] : 0, t >= 2 ? u[t - 2] : 0]);
const baselineAcc = accuracy(rawFeatures);

console.log(`  fixed random reservoir + trained linear readout : ${(reservoirAcc * 100).toFixed(1)}%`);
console.log(`  linear readout on the raw delayed inputs        : ${(baselineAcc * 100).toFixed(1)}%  (≈ chance — XOR isn't linearly separable)`);
console.log('\n→ the reservoir (untrained!) supplies the nonlinear temporal features;');
console.log('  only the linear readout learns. The medium did the hard part for free.\n');
