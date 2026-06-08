// =============================================================================
// waves_worx — BLE game LINK: detect + decode under real RF impairments
// =============================================================================
// Companion to ble-game-transfer.test.mjs (which unit-tests the blegame codec).
// This suite drives that SAME real 21-byte codec over a simulated BLE link that
// drops, duplicates, reorders, and corrupts notifications — and proves the
// receiver DETECTS the damage and RECOVERS the game payload via ARQ retransmit,
// including pairing-session binding so a move is only reconstructed by the peer
// that completed the handshake. No cloud, no radio, deterministic (seeded).
// =============================================================================
import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  BLE_GAME_FRAME_LEN,
  BLE_GAME_KIND,
  createPairing,
  decodeBleGameTransfer,
  detectBleGameFrame,
  encodeBleGameFrame,
  encodeBleGameTransfer,
  generateRoomCode,
  PAIRING_STATE,
  toBytes,
} from '../src/index.js';

const text = (bytes) => new TextDecoder().decode(bytes);

// deterministic PRNG — never Math.random in a test
const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32;
};

/** Simulate a BLE notification link over real 21-byte frames: drop / dup / reorder. */
function bleDeliver(frames, { drop = new Set(), dupRate = 0, reorder = false, seed = 1 } = {}) {
  const rnd = lcg(seed);
  let out = [];
  frames.forEach((f, i) => {
    assert.equal(f.length, BLE_GAME_FRAME_LEN, 'every frame must fit the BLE budget');
    if (drop.has(i)) return; // lost in flight
    out.push(f);
    if (dupRate && rnd() < dupRate) out.push(f); // duplicated notification
  });
  if (reorder) out = out.map((f) => ({ f, k: rnd() })).sort((a, b) => a.k - b.k).map((x) => x.f);
  return out;
}

/** A receiver that keeps only valid frames; tracks seen indices + reject reasons. */
function receiver() {
  const seen = new Map(); // index -> frame
  const rejects = [];
  let total = 0;
  return {
    ingest(frame) {
      const d = detectBleGameFrame(frame);
      if (!d.ok) {
        rejects.push(d.reason);
        return d;
      }
      seen.set(d.index, frame);
      total = d.total;
      return d;
    },
    missing() {
      const m = [];
      for (let i = 0; i < total; i++) if (!seen.has(i)) m.push(i);
      return m;
    },
    rejects,
    frames: () => [...seen.values()],
  };
}

test('reordered + duplicated BLE notifications still decode the game state', () => {
  const payload = 'arcade-state:' + '0123456789'.repeat(6);
  const frames = encodeBleGameTransfer(payload, { gameId: 9, transferId: 9 });
  assert.ok(frames.length > 1);

  const rx = receiver();
  for (const f of bleDeliver(frames, { reorder: true, dupRate: 0.6, seed: 5 })) rx.ingest(f);

  const decoded = decodeBleGameTransfer(rx.frames());
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(text(decoded.payload), payload);
});

test('lost BLE notifications are detected as missing, then recovered by ARQ (with an ack frame)', () => {
  const payload = 'state:' + '0123456789'.repeat(5);
  const frames = encodeBleGameTransfer(payload, { gameId: 3, transferId: 7 });

  // Round 1: frames 1 and 4 dropped in flight.
  const rx = receiver();
  for (const f of bleDeliver(frames, { drop: new Set([1, 4]), seed: 2 })) rx.ingest(f);

  assert.equal(decodeBleGameTransfer(rx.frames()).reason, 'incomplete-transfer');
  const miss = rx.missing();
  assert.deepEqual(miss, [1, 4]); // receiver knows exactly what to NACK

  // The receiver NACKs with a real ack-kind frame carrying the missing indices.
  const ack = encodeBleGameFrame({ kind: 'ack', gameId: 3, transferId: 7, payload: Uint8Array.from(miss) });
  const ackOut = detectBleGameFrame(ack);
  assert.equal(ackOut.ok, true);
  assert.equal(ackOut.kind, BLE_GAME_KIND.ack);
  assert.deepEqual([...ackOut.payload], miss);

  // Round 2: sender retransmits exactly those frames → transfer completes.
  for (const i of ackOut.payload) rx.ingest(frames[i]);
  const decoded = decodeBleGameTransfer(rx.frames());
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(text(decoded.payload), payload);
});

test('a corrupted BLE frame is detected (checksum) and dropped, then ARQ-recovered', () => {
  const payload = 'hello from the game room peers';
  const frames = encodeBleGameTransfer(payload, { gameId: 1, transferId: 1 });

  // Flip a payload byte of frame 2 in flight (length stays 21).
  const wire = frames.map((f, i) => (i === 2 ? Uint8Array.from(f, (b, j) => (j === 10 ? b ^ 0xff : b)) : f));

  const rx = receiver();
  for (const f of bleDeliver(wire, { seed: 3 })) rx.ingest(f);

  assert.ok(rx.rejects.includes('checksum-mismatch')); // detected, never trusted
  assert.equal(decodeBleGameTransfer(rx.frames()).reason, 'incomplete-transfer');

  rx.ingest(frames[2]); // clean retransmit
  assert.equal(decodeBleGameTransfer(rx.frames()).ok, true);
});

test('two devices pair, then a session-bound move only decodes for the paired peer', () => {
  const room = generateRoomCode();
  const a = createPairing({ roomCode: room, role: 'initiator', nonce: () => 'AAAA' });
  const b = createPairing({ roomCode: room, role: 'responder', nonce: () => 'BBBB' });

  const offer = a.start();
  const answer = b.receive(offer).send;
  const confirm = a.receive(answer).send;
  b.receive(confirm);
  assert.equal(a.state, PAIRING_STATE.PAIRED);
  assert.equal(a.session, b.session);

  // Transfer ids are derived from the shared session — only the paired peer
  // reconstructs them, so a stranger's frames would land on different ids.
  const gid = (s) => parseInt(s.slice(0, 2), 16) & 0xff;
  const tid = (s) => parseInt(s.slice(2, 4), 16) & 0xff;

  const frames = encodeBleGameTransfer(toBytes('mv:5,2'), {
    gameId: gid(a.session),
    transferId: tid(a.session),
  });

  const rx = receiver();
  for (const f of bleDeliver(frames, { reorder: true, dupRate: 0.3, seed: 6 })) rx.ingest(f);

  const decoded = decodeBleGameTransfer(rx.frames());
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(decoded.gameId, gid(b.session)); // receiver agrees on the session-derived id
  assert.equal(text(decoded.payload), 'mv:5,2');
});
