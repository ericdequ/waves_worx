// =============================================================================
// waves_worx — frame: bytes in, bytes out, over any channel
// =============================================================================
// The universal byte layer every physical/network transport in the lab speaks.
// Coerce anything Uint8Array-ish to bytes, round-trip through base64 (for QR /
// text channels), and JSON-encode small control frames. No cloud, no deps.
// Generalized from BEV's bevnet frame helpers.
// =============================================================================

const enc = () => new TextEncoder();
const dec = () => new TextDecoder();

/** Coerce a Uint8Array | ArrayBuffer | TypedArray | number[] | string to bytes. */
export function toBytes(value) {
  if (value instanceof Uint8Array) return value;
  if (value instanceof ArrayBuffer) return new Uint8Array(value);
  if (ArrayBuffer.isView(value)) return new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
  if (Array.isArray(value)) return Uint8Array.from(value);
  if (typeof value === 'string') return enc().encode(value);
  throw new TypeError('waves_worx: frame bytes must be Uint8Array-compatible');
}

/** Defensive copy of bytes (so a channel can't mutate a caller's buffer). */
export const cloneBytes = (value) => new Uint8Array(toBytes(value));

export const encodeJSON = (obj) => enc().encode(JSON.stringify(obj ?? {}));
export const decodeJSON = (bytes) => JSON.parse(dec().decode(toBytes(bytes)));

export function bytesToBase64(value) {
  const bytes = toBytes(value);
  if (typeof Buffer !== 'undefined') return Buffer.from(bytes).toString('base64');
  let binary = '';
  bytes.forEach((b) => (binary += String.fromCharCode(b)));
  return btoa(binary);
}

export function base64ToBytes(value) {
  const raw = String(value || '');
  if (typeof Buffer !== 'undefined') return new Uint8Array(Buffer.from(raw, 'base64'));
  const binary = atob(raw);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i);
  return out;
}

/** URL-safe base64 (for QR payloads / join links). */
export const bytesToBase64Url = (value) =>
  bytesToBase64(value).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/u, '');

export function base64UrlToBytes(value) {
  const raw = String(value || '').replaceAll('-', '+').replaceAll('_', '/');
  return base64ToBytes(raw.padEnd(raw.length + ((4 - (raw.length % 4)) % 4), '='));
}

/**
 * CRC-32 (IEEE) over bytes — a cheap integrity check for lossy physical
 * channels. Returns an unsigned 32-bit number.
 */
export function crc32(value) {
  const bytes = toBytes(value);
  let crc = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) {
    crc ^= bytes[i];
    for (let bit = 0; bit < 8; bit++) {
      crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1;
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}
