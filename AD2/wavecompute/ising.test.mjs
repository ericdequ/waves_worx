// =============================================================================
// AD2/wavecompute — tests. The oscillator Ising machine must find the SAME
// optimum as brute force on MAX-CUT — wave dynamics matching the exact answer.
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { isingEnergy, maxCut, solveIsing } from './ising.js';

/** Ground-truth MAX-CUT by exhaustive enumeration (n ≤ ~16). */
function bruteMaxCut(edges, n) {
  let best = 0;
  for (let m = 0; m < 1 << n; m++) {
    let cut = 0;
    for (const [a, b, w = 1] of edges) if (((m >> a) & 1) !== ((m >> b) & 1)) cut += w;
    if (cut > best) best = cut;
  }
  return best;
}

const C4 = { n: 4, edges: [[0, 1], [1, 2], [2, 3], [3, 0]] };            // square: optimum 4 (bipartite)
const K3 = { n: 3, edges: [[0, 1], [1, 2], [2, 0]] };                    // triangle: optimum 2 (frustrated)
const C5 = { n: 5, edges: [[0, 1], [1, 2], [2, 3], [3, 4], [4, 0]] };    // pentagon: optimum 4
const K4 = { n: 4, edges: [[0, 1], [0, 2], [0, 3], [1, 2], [1, 3], [2, 3]] }; // optimum 4
const RANDOM8 = {
  n: 8,
  edges: [[0, 1], [0, 3], [1, 2], [1, 5], [2, 6], [3, 4], [4, 5], [4, 7], [5, 6], [6, 7], [0, 7], [2, 4]],
};

for (const [name, g] of Object.entries({ C4, K3, C5, K4, RANDOM8 })) {
  test(`Ising machine finds the optimal MAX-CUT on ${name}`, () => {
    const optimum = bruteMaxCut(g.edges, g.n);
    const { cut, partition } = maxCut(g.edges, g.n, { seed: 7 });
    assert.equal(cut, optimum, `${name}: machine cut ${cut} vs optimum ${optimum}`);
    assert.equal(partition[0].length + partition[1].length, g.n);
  });
}

test('the recovered spins really achieve the reported (minimum) energy', () => {
  // J = -W for K4; the machine's energy must equal the energy of its own spins.
  const J = Array.from({ length: 4 }, () => new Float64Array(4));
  for (const [a, b] of K4.edges) { J[a][b] -= 1; J[b][a] -= 1; }
  const { spins, energy } = solveIsing(J, { seed: 3 });
  assert.ok(Math.abs(isingEnergy(J, spins) - energy) < 1e-9);
});

test('a frustrated triangle cannot cut all edges — the machine respects that', () => {
  // K3 optimum is 2, never 3: physics finds the best *achievable*, not the wished-for.
  assert.equal(maxCut(K3.edges, K3.n, { seed: 1 }).cut, 2);
  assert.equal(bruteMaxCut(K3.edges, K3.n), 2);
});
