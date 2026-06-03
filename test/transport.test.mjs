import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  toBytes,
  crc32,
  bytesToBase64Url,
  base64UrlToBytes,
  chunk,
  Reassembler,
  createLoopbackBus,
  createLoopbackChannel,
  pickChannel,
} from '../src/index.js';

test('frame: base64url round-trips arbitrary bytes', () => {
  const bytes = Uint8Array.from([0, 255, 1, 254, 128, 64]);
  assert.deepEqual([...base64UrlToBytes(bytesToBase64Url(bytes))], [...bytes]);
});

test('frame: crc32 matches the IEEE check value for "123456789"', () => {
  assert.equal(crc32('123456789'), 0xcbf43926);
});

test('chunk + reassemble: out-of-order delivery rebuilds the payload', () => {
  const payload = toBytes('the quick brown fox jumps over the lazy dog'.repeat(20));
  const frames = chunk(payload, { id: 'x', size: 16 });
  assert.ok(frames.length > 1);

  const r = new Reassembler();
  // Shuffle + duplicate a frame; integrity + ordering must still hold.
  const shuffled = [...frames].reverse();
  shuffled.splice(3, 0, frames[0]); // duplicate
  let status;
  for (const f of shuffled) status = r.add(f);
  assert.equal(status.complete, true);
  assert.deepEqual([...r.bytes()], [...payload]);
});

test('chunk: a CRC-corrupted frame is rejected and reported missing', () => {
  const frames = chunk('hello world hello world', { size: 8 });
  const r = new Reassembler();
  for (const f of frames) {
    if (f.index === 1) {
      const bad = { ...f, bytes: Uint8Array.from([...f.bytes].map((b) => b ^ 0xff)) };
      assert.equal(r.add(bad).reason, 'crc-mismatch');
    } else {
      r.add(f);
    }
  }
  assert.equal(r.complete, false);
  assert.deepEqual(r.missing(), [1]);
});

test('channel: two loopback channels on one bus exchange frames (no cloud)', () => {
  const bus = createLoopbackBus();
  const a = createLoopbackChannel({ id: 'a', bus });
  const b = createLoopbackChannel({ id: 'b', bus });

  const got = [];
  b.subscribe((bytes, meta) => got.push({ text: new TextDecoder().decode(bytes), from: meta.fromId }));
  const delivered = a.send(toBytes('ping'));

  assert.equal(delivered, 1); // delivered to b, not echoed to a
  assert.deepEqual(got, [{ text: 'ping', from: 'a' }]);
  assert.equal(a.diagnostics().sent, 1);
});

test('channel: a full chunked transfer survives a loopback hop', () => {
  const bus = createLoopbackBus();
  const sender = createLoopbackChannel({ id: 's', bus });
  const receiver = createLoopbackChannel({ id: 'r', bus });

  const r = new Reassembler();
  receiver.subscribe((bytes) => r.add(JSON.parse(new TextDecoder().decode(bytes))));

  const payload = toBytes('SECRET-PAYLOAD-' + 'x'.repeat(500));
  for (const f of chunk(payload, { id: 'job', size: 64 })) {
    // serialize the frame as JSON-with-array-bytes for the wire
    sender.send(new TextEncoder().encode(JSON.stringify({ ...f, bytes: [...f.bytes] })));
  }
  assert.equal(r.complete, true);
  assert.deepEqual([...r.bytes()], [...payload]);
});

test('channel: pickChannel honors the preference ladder', () => {
  assert.equal(pickChannel(['qr', 'ble', 'loopback']), 'ble');
  assert.equal(pickChannel(['qr', 'loopback']), 'qr');
  assert.equal(pickChannel(['nope']), null);
});
