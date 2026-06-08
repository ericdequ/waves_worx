#!/usr/bin/env node
// =============================================================================
// AD2/wavecompute demo — an oscillator Ising machine solving MAX-CUT.
//   node demo.mjs
// =============================================================================
import { maxCut } from './ising.js';

// A small graph (an 8-node ring with chords). MAX-CUT is NP-hard in general —
// the same problem class quantum annealers target.
const n = 8;
const edges = [
  [0, 1], [1, 2], [2, 3], [3, 4], [4, 5], [5, 6], [6, 7], [7, 0], // ring
  [0, 4], [1, 5], [2, 6], [3, 7], // chords
];

console.log(`graph: ${n} nodes, ${edges.length} edges\n`);
console.log('running the oscillator network — pump ramps, oscillators bifurcate…\n');

const { cut, partition } = maxCut(edges, n, { seed: 7 });

console.log(`partition A = {${partition[0].join(', ')}}`);
console.log(`partition B = {${partition[1].join(', ')}}`);
console.log(`\nedges cut: ${cut} / ${edges.length}`);
console.log(
  'each cut edge crosses between A and B — the network settled into the ' +
    'max-cut by relaxation, not by enumerating all 2^8 configurations.\n',
);
