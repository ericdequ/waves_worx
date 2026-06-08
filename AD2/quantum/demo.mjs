#!/usr/bin/env node
// =============================================================================
// AD2/quantum demo — Grover's search, shown as wave modes.   node demo.mjs
// =============================================================================
import { QState } from './qwave.js';

const show = (q, title) => {
  console.log(`\n${title}`);
  for (const m of q.waveModes()) {
    const bar = '█'.repeat(Math.round(m.power * 30));
    console.log(`  |${m.bits}⟩  ${m.freqHz}Hz  power ${m.power.toFixed(3)} ${bar}`);
  }
};

const MARKED = 0b10; // search for the item |10⟩ among 4

const q = new QState(2).hAll(); // uniform superposition — all four tones equal
show(q, '1) superposition: four equal wave modes (we know nothing yet)');

q.oracle((i) => i === MARKED); // phase-flip the marked mode (invisible to power)
show(q, '2) oracle marks |10⟩ by flipping its phase — power unchanged, phase hidden');

q.diffuse(); // inversion about the mean — interference amplifies the marked mode
show(q, '3) diffusion: interference collapses all energy onto the answer');

console.log(`\n→ Grover found |${MARKED.toString(2).padStart(2, '0')}⟩ with certainty in one iteration.\n`);
