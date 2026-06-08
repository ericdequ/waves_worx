// =============================================================================
// waves_worx — blegame: BLE-sized game data transfer frames
// =============================================================================
// Pure testable codec for BLE manufacturer-data-sized game payloads. It is not a
// native BLE adapter; it models the bytes that a native BLE advertiser/scanner
// can carry. Frame length is fixed at 21 bytes to match the Go BLE contract.
// =============================================================================

import { crc32, toBytes } from './frame.js';

export const BLE_GAME_FRAME_LEN = 21;
export const BLE_GAME_PAYLOAD_BYTES = 10;
export const BLE_GAME_VERSION = 1;
export const BLE_GAME_MAGIC = Object.freeze([0x57, 0x47]); // "WG" / waves game

export const BLE_GAME_KIND = Object.freeze({
  data: 1,
  ack: 2,
  state: 3,
});

const u8 = (value) => Number(value) & 0xff;

const checksum16 = (bytes) => crc32(bytes) & 0xffff;

const normalizeKind = (kind) => {
  if (typeof kind === 'number') return u8(kind);
  return BLE_GAME_KIND[kind] || BLE_GAME_KIND.data;
};

/**
 * Build a single fixed-size BLE game frame.
 * Layout:
 *   [0..1] magic "WG"
 *   [2]    version
 *   [3]    kind
 *   [4]    game id
 *   [5]    transfer id
 *   [6]    index
 *   [7]    total
 *   [8]    payload length
 *   [9..18] payload (zero padded)
 *   [19..20] crc16 over [0..18]
 */
export function encodeBleGameFrame({
  kind = 'data',
  gameId = 0,
  transferId = 0,
  index = 0,
  total = 1,
  payload = new Uint8Array(),
} = {}) {
  const body = toBytes(payload);
  if (body.length > BLE_GAME_PAYLOAD_BYTES) {
    throw new RangeError(`BLE game payload max is ${BLE_GAME_PAYLOAD_BYTES} bytes`);
  }
  const frame = new Uint8Array(BLE_GAME_FRAME_LEN);
  frame[0] = BLE_GAME_MAGIC[0];
  frame[1] = BLE_GAME_MAGIC[1];
  frame[2] = BLE_GAME_VERSION;
  frame[3] = normalizeKind(kind);
  frame[4] = u8(gameId);
  frame[5] = u8(transferId);
  frame[6] = u8(index);
  frame[7] = u8(total);
  frame[8] = body.length;
  frame.set(body, 9);
  const sum = checksum16(frame.subarray(0, 19));
  frame[19] = (sum >>> 8) & 0xff;
  frame[20] = sum & 0xff;
  return frame;
}

/** Detect and decode a BLE game frame. Returns `{ok:false, reason}` on reject. */
export function detectBleGameFrame(bytes) {
  const frame = toBytes(bytes);
  if (frame.length !== BLE_GAME_FRAME_LEN) {
    return { ok: false, reason: 'bad-length' };
  }
  if (frame[0] !== BLE_GAME_MAGIC[0] || frame[1] !== BLE_GAME_MAGIC[1]) {
    return { ok: false, reason: 'bad-magic' };
  }
  if (frame[2] !== BLE_GAME_VERSION) {
    return { ok: false, reason: 'bad-version' };
  }
  const payloadLen = frame[8];
  if (payloadLen > BLE_GAME_PAYLOAD_BYTES) {
    return { ok: false, reason: 'bad-payload-length' };
  }
  const got = (frame[19] << 8) | frame[20];
  const want = checksum16(frame.subarray(0, 19));
  if (got !== want) {
    return { ok: false, reason: 'checksum-mismatch' };
  }
  return {
    ok: true,
    kind: frame[3],
    gameId: frame[4],
    transferId: frame[5],
    index: frame[6],
    total: frame[7],
    payload: frame.slice(9, 9 + payloadLen),
  };
}

/** Split a game payload into BLE-sized data frames. */
export function encodeBleGameTransfer(payload, { gameId = 0, transferId = 1, kind = 'data' } = {}) {
  const bytes = toBytes(payload);
  const total = Math.max(1, Math.ceil(bytes.length / BLE_GAME_PAYLOAD_BYTES));
  if (total > 255) throw new RangeError('BLE game transfer max is 255 frames');
  const frames = [];
  for (let index = 0; index < total; index++) {
    frames.push(
      encodeBleGameFrame({
        kind,
        gameId,
        transferId,
        index,
        total,
        payload: bytes.subarray(
          index * BLE_GAME_PAYLOAD_BYTES,
          index * BLE_GAME_PAYLOAD_BYTES + BLE_GAME_PAYLOAD_BYTES,
        ),
      }),
    );
  }
  return frames;
}

/** Decode a complete set of BLE game frames. Frames may arrive out of order. */
export function decodeBleGameTransfer(frames) {
  const decoded = [];
  for (const frame of frames) {
    const item = detectBleGameFrame(frame);
    if (!item.ok) return item;
    decoded.push(item);
  }
  if (!decoded.length) return { ok: false, reason: 'empty-transfer' };
  const first = decoded[0];
  const sameTransfer = decoded.every(
    (item) =>
      item.gameId === first.gameId &&
      item.transferId === first.transferId &&
      item.total === first.total,
  );
  if (!sameTransfer) return { ok: false, reason: 'mixed-transfer' };
  const byIndex = new Map(decoded.map((item) => [item.index, item]));
  if (byIndex.size !== first.total) return { ok: false, reason: 'incomplete-transfer' };
  const parts = [];
  let length = 0;
  for (let index = 0; index < first.total; index++) {
    const item = byIndex.get(index);
    if (!item) return { ok: false, reason: 'missing-frame', missing: index };
    parts.push(item.payload);
    length += item.payload.length;
  }
  const payload = new Uint8Array(length);
  let offset = 0;
  for (const part of parts) {
    payload.set(part, offset);
    offset += part.length;
  }
  return {
    ok: true,
    kind: first.kind,
    gameId: first.gameId,
    transferId: first.transferId,
    total: first.total,
    payload,
  };
}
