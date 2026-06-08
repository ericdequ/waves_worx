// =============================================================================
// AD2/wavecompute — reservoir computing tests. The classic demonstration:
// a temporal task (delayed XOR) that needs BOTH memory and nonlinearity. A
// fixed random reservoir + a trained linear readout solves it; a linear readout
// on the raw inputs cannot (XOR isn't linearly separable). That gap IS the point.
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { createReservoir, dot, ridgeFit, runStream, stateFeature } from './reservoir.js';

const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32;
};

// Temporal task: y(t) = u(t-1) XOR u(t-2). Needs 2-step memory AND nonlinearity.
function buildTask(T, seed) {
  const rnd = lcg(seed);
  const u = Array.from({ length: T }, () => (rnd() < 0.5 ? 0 : 1));
  const y = u.map((_, t) => (t >= 2 ? ((u[t - 1] ^ u[t - 2]) ? 1 : -1) : 0));
  return { u, y };
}

function evaluate(features, y, washout) {
  const idx = [];
  for (let t = washout; t < y.length; t++) idx.push(t);
  const X = idx.map((t) => features[t]);
  const Y = idx.map((t) => y[t]);
  const split = Math.floor(idx.length * 0.7);
  const w = ridgeFit(X.slice(0, split), Y.slice(0, split), 1e-3);
  let correct = 0;
  for (let i = split; i < idx.length; i++) if ((dot(w, X[i]) >= 0 ? 1 : -1) === Y[i]) correct++;
  return correct / (idx.length - split);
}

test('reservoir computing solves the delayed-XOR temporal task (>90% on held-out)', () => {
  const T = 1200;
  const { u, y } = buildTask(T, 42);
  const res = createReservoir({ nIn: 1, nRes: 80, spectralRadius: 0.92, inputScale: 1, seed: 7 });
  const states = runStream(res, u.map((x) => [x]));
  const acc = evaluate(states.map(stateFeature), y, 60);
  assert.ok(acc > 0.9, `reservoir accuracy ${acc.toFixed(3)} should exceed 0.9`);
});

test('the reservoir is necessary: a linear readout on raw inputs cannot (XOR is not separable)', () => {
  const T = 1200;
  const { u, y } = buildTask(T, 42);

  const res = createReservoir({ nIn: 1, nRes: 80, spectralRadius: 0.92, inputScale: 1, seed: 7 });
  const reservoirAcc = evaluate(runStream(res, u.map((x) => [x])).map(stateFeature), y, 60);

  // Baseline: linear readout given the raw delayed inputs [1, u(t-1), u(t-2)].
  const rawFeatures = u.map((_, t) => [1, t >= 1 ? u[t - 1] : 0, t >= 2 ? u[t - 2] : 0]);
  const baselineAcc = evaluate(rawFeatures, y, 60);

  assert.ok(reservoirAcc > 0.9, `reservoir ${reservoirAcc.toFixed(3)}`);
  assert.ok(baselineAcc < 0.85, `linear-on-raw ${baselineAcc.toFixed(3)} should be near chance for XOR`);
  assert.ok(reservoirAcc - baselineAcc > 0.1, 'the reservoir must clearly beat the linear baseline');
});
