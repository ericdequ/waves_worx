import { test } from 'node:test';
import assert from 'node:assert/strict';

import { encode, decode, modemBand, toBytes } from '../src/index.js';

test('modem: encode → decode round-trips a payload', () => {
  const tones = encode('hello waves');
  assert.ok(tones.length > 0);
  assert.ok(tones.every((t) => typeof t.freqHz === 'number' && t.durationMs > 0));
  const out = decode(tones.map((t) => t.freqHz));
  assert.equal(out.ok, true);
  assert.equal(new TextDecoder().decode(out.bytes), 'hello waves');
});

test('modem: tolerates frequency jitter (nearest-symbol quantization)', () => {
  const tones = encode(Uint8Array.from([0x00, 0x7f, 0xff, 0xa5]));
  // ±30 Hz jitter — well under the 80 Hz step, so symbols still resolve.
  const jitter = (i) => (i % 2 === 0 ? 25 : -25);
  const observed = tones.map((t, i) => t.freqHz + jitter(i));
  const out = decode(observed);
  assert.equal(out.ok, true);
  assert.deepEqual([...out.bytes], [0x00, 0x7f, 0xff, 0xa5]);
});

test('modem: rejects a stream with no preamble', () => {
  const out = decode([1200, 1300, 1400]);
  assert.equal(out.ok, false);
  assert.equal(out.reason, 'no-preamble');
});

test('modem: detects corruption via checksum', () => {
  const tones = encode(toBytes('integrity'));
  const freqs = tones.map((t) => t.freqHz);
  // Corrupt one data tone by a full symbol step (past the preamble).
  freqs[6] += 80;
  const out = decode(freqs);
  assert.equal(out.ok, false);
  assert.equal(out.reason, 'checksum-mismatch');
});

test('modem: band stays inside the audible carrier window', () => {
  const { lowHz, highHz } = modemBand();
  assert.equal(lowHz, 1200);
  assert.equal(highHz, 1200 + 15 * 80); // 2400 Hz
});
