// =============================================================================
// AD2/quantum — tests. Real quantum algorithms running as wave interference:
// superposition, entanglement (Bell), Grover search, and the unitarity invariant.
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { QState } from './qwave.js';

const close = (a, b, eps = 1e-9) => Math.abs(a - b) < eps;
const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32;
};

test('X is a NOT gate: |0⟩ → |1⟩', () => {
  const p = new QState(1).x(0).probs();
  assert.ok(close(p[0], 0) && close(p[1], 1));
});

test('Hadamard makes a 50/50 superposition (two equal wave modes)', () => {
  const q = new QState(1).h(0);
  const p = q.probs();
  assert.ok(close(p[0], 0.5) && close(p[1], 0.5));
  // as waves: two tones of equal magnitude, in phase
  const modes = q.waveModes();
  assert.equal(modes.length, 2);
  assert.ok(close(modes[0].amplitude, modes[1].amplitude));
});

test('H then H returns to |0⟩ (interference cancels — the wave un-mixes)', () => {
  const p = new QState(1).h(0).h(0).probs();
  assert.ok(close(p[0], 1) && close(p[1], 0));
});

test('Bell state H·CNOT entangles: only |00⟩ and |11⟩ have power', () => {
  const q = new QState(2).h(0).cnot(0, 1);
  const p = q.probs();
  assert.ok(close(p[0b00], 0.5));
  assert.ok(close(p[0b11], 0.5));
  assert.ok(close(p[0b01], 0) && close(p[0b10], 0));
});

test('Bell state is correlated: every measurement gives matching bits', () => {
  const rng = lcg(12345);
  for (let shot = 0; shot < 200; shot++) {
    const outcome = new QState(2).h(0).cnot(0, 1).sample(rng);
    const bit0 = outcome & 1;
    const bit1 = (outcome >> 1) & 1;
    assert.equal(bit0, bit1, `shot ${shot}: bits must agree (entanglement)`);
  }
});

test("Grover finds the marked item in 2 qubits with one iteration (certainty)", () => {
  for (let marked = 0; marked < 4; marked++) {
    const q = new QState(2).hAll(); // uniform superposition over 4 states
    q.oracle((i) => i === marked).diffuse(); // one Grover iteration
    const p = q.probs();
    assert.ok(close(p[marked], 1, 1e-9), `marked ${marked} should be certain, got ${p[marked]}`);
  }
});

test('unitarity invariant: total wave power stays 1 through any circuit', () => {
  const q = new QState(3).h(0).t(1).cnot(0, 2).phase(1, 0.7).y(2).cnot(1, 0).hAll();
  const total = q.probs().reduce((a, b) => a + b, 0);
  assert.ok(close(total, 1, 1e-9), `total power ${total}`);
});

test('a relative phase is invisible to power but real in the wave modes', () => {
  // Z on |+⟩ flips the relative phase: same 50/50 power, opposite phase.
  const plus = new QState(1).h(0);
  const minus = new QState(1).h(0).z(0);
  assert.ok(close(plus.probs()[1], minus.probs()[1])); // power identical
  const dPhase = minus.waveModes()[1].phaseRad - plus.waveModes()[1].phaseRad;
  assert.ok(close(Math.abs(dPhase), Math.PI), `phase shift ${dPhase} should be π`);
});
