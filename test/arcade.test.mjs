import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  createProximityProof,
  generateRoomCode,
  isRoomCode,
  normalizeRoomCode,
  verifyProximityProof,
} from '../src/index.js';

test('room code: normalize strips noise and caps at 6', () => {
  assert.equal(normalizeRoomCode(' ab-cd ef gh '), 'ABCDEF');
  assert.equal(normalizeRoomCode('xy'), 'XY');
  assert.equal(isRoomCode('ABC123'), true);
  assert.equal(isRoomCode('ABC'), false);
});

test('room code: generation is deterministic under an injected RNG', () => {
  let i = 0;
  const seq = [0, 0.99, 0.5, 0.1, 0.9, 0.3];
  const code = generateRoomCode(() => seq[i++ % seq.length]);
  assert.equal(code.length, 6);
  assert.equal(code, generateRoomCode((() => { let j = 0; return () => seq[j++ % seq.length]; })()));
});

test('proximity: a faithfully-observed pattern verifies', async () => {
  const proof = await createProximityProof({ roomCode: 'ABC123', token: 'session-token-xyz' });
  // The observer reproduces exactly the displayed bits.
  const result = await verifyProximityProof({ roomCode: 'ABC123', token: 'session-token-xyz', observedBits: proof.bits });
  assert.equal(result.ok, true);
  assert.equal(result.reason, 'verified');
  assert.equal(result.confidence, 1);
});

test('proximity: tolerates a noisy observation above threshold but rejects below', async () => {
  const proof = await createProximityProof({ roomCode: 'ABC123', token: 'tok' });
  const flip = (bits, n) => {
    const out = [...bits];
    for (let k = 0; k < n; k++) out[k * 3 % out.length] ^= 1;
    return out;
  };
  // A couple of misread frames — still above the 0.9 default.
  const noisy = await verifyProximityProof({ roomCode: 'ABC123', token: 'tok', observedBits: flip(proof.bits, 1) });
  assert.equal(noisy.ok, true);
  // Heavy corruption — below threshold.
  const garbage = await verifyProximityProof({ roomCode: 'ABC123', token: 'tok', observedBits: flip(proof.bits, 12) });
  assert.equal(garbage.ok, false);
  assert.equal(garbage.reason, 'insufficient-pattern-match');
});

test('proximity: wrong token cannot reproduce the pattern (anti-spoof)', async () => {
  const proof = await createProximityProof({ roomCode: 'ABC123', token: 'real-token' });
  // An attacker who knows the room code but not the session token, replaying a
  // generic all-on pattern, fails.
  const spoof = await verifyProximityProof({ roomCode: 'ABC123', token: 'wrong-token', observedBits: proof.bits });
  assert.equal(spoof.ok, false);
});

test('proximity: a 6-char room code is required', async () => {
  await assert.rejects(createProximityProof({ roomCode: 'AB', token: 't' }), /6-character/);
  await assert.rejects(createProximityProof({ roomCode: 'ABC123' }), /session token/);
});
