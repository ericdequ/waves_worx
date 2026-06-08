import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  BLE_GAME_FRAME_LEN,
  BLE_GAME_PAYLOAD_BYTES,
  createLoopbackBus,
  createLoopbackChannel,
  decodeBleGameTransfer,
  detectBleGameFrame,
  encodeBleGameFrame,
  encodeBleGameTransfer,
} from '../src/index.js';

const text = (bytes) => new TextDecoder().decode(bytes);

test('BLE game frame encodes into the 21-byte manufacturer-data budget', () => {
  const frame = encodeBleGameFrame({
    gameId: 7,
    transferId: 9,
    index: 2,
    total: 5,
    payload: 'roll',
  });

  assert.equal(frame.length, BLE_GAME_FRAME_LEN);
  const decoded = detectBleGameFrame(frame);
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(decoded.gameId, 7);
  assert.equal(decoded.transferId, 9);
  assert.equal(decoded.index, 2);
  assert.equal(decoded.total, 5);
  assert.equal(text(decoded.payload), 'roll');
});

test('BLE game frame detector rejects bad magic, length, payload length, and checksum', () => {
  const frame = encodeBleGameFrame({ payload: 'abc' });

  assert.equal(detectBleGameFrame(frame.subarray(1)).reason, 'bad-length');

  const badMagic = new Uint8Array(frame);
  badMagic[0] = 0;
  assert.equal(detectBleGameFrame(badMagic).reason, 'bad-magic');

  const badPayloadLen = new Uint8Array(frame);
  badPayloadLen[8] = BLE_GAME_PAYLOAD_BYTES + 1;
  const payloadLenSum = encodeBleGameFrame({ payload: 'abc' });
  payloadLenSum[8] = badPayloadLen[8];
  payloadLenSum[19] = 0;
  payloadLenSum[20] = 0;
  assert.equal(detectBleGameFrame(payloadLenSum).reason, 'bad-payload-length');

  const tampered = new Uint8Array(frame);
  tampered[9] ^= 0xff;
  assert.equal(detectBleGameFrame(tampered).reason, 'checksum-mismatch');
});

test('BLE game transfer decodes out-of-order frames into one payload', () => {
  const payload = 'arcade-state:' + '0123456789'.repeat(8);
  const frames = encodeBleGameTransfer(payload, { gameId: 4, transferId: 44 });
  assert.ok(frames.length > 1);
  assert.ok(frames.every((frame) => frame.length === BLE_GAME_FRAME_LEN));

  const decoded = decodeBleGameTransfer([...frames].reverse());
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(decoded.gameId, 4);
  assert.equal(decoded.transferId, 44);
  assert.equal(text(decoded.payload), payload);
});

test('BLE game transfer reports incomplete or mixed frame sets', () => {
  const frames = encodeBleGameTransfer('hello from game room', { gameId: 1, transferId: 2 });
  assert.equal(decodeBleGameTransfer(frames.slice(1)).reason, 'incomplete-transfer');

  const other = encodeBleGameTransfer('other', { gameId: 2, transferId: 2 });
  assert.equal(decodeBleGameTransfer([frames[0], other[0]]).reason, 'mixed-transfer');
});

test('BLE game frames survive loopback channel delivery and decode', () => {
  const bus = createLoopbackBus();
  const sender = createLoopbackChannel({ id: 'phone-a', bus });
  const receiver = createLoopbackChannel({ id: 'phone-b', bus });
  const inbox = [];

  receiver.subscribe((bytes, meta) => {
    const frame = detectBleGameFrame(bytes);
    if (frame.ok) inbox.push({ frame: bytes, fromId: meta.fromId });
  });

  const frames = encodeBleGameTransfer('join-room:K7QF9M:ride-the-bus', {
    gameId: 12,
    transferId: 88,
  });
  for (const frame of frames) sender.send(frame, { channel: 'ble' });

  assert.equal(inbox.length, frames.length);
  assert.ok(inbox.every((item) => item.fromId === 'phone-a'));
  const decoded = decodeBleGameTransfer(inbox.map((item) => item.frame));
  assert.equal(decoded.ok, true, decoded.reason);
  assert.equal(text(decoded.payload), 'join-room:K7QF9M:ride-the-bus');
});
