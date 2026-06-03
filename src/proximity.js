// =============================================================================
// waves_worx — proximity proof: prove two devices are physically co-present
// =============================================================================
// The arcade's headline demo of physical-layer security. One device DISPLAYS a
// light/screen pattern (a bit sequence: preamble + secret-derived data bits +
// checksum); the other OBSERVES the flashes with its camera. Because the secret
// is derived from a shared room code + session token, only a device actually
// watching the screen can reproduce the bits — you cannot pass the proof from
// across the internet. This is anti-spoofing for CONSENSUAL pairing, not covert
// signalling: both parties opt in, nothing leaves the room.
//
// Generalized (de-app-specific) from BEV's projected-QR verification, which
// shipped with a parity test suite. Uses Web Crypto SHA-256 when present, with
// a deterministic non-crypto fallback so it runs anywhere.
// =============================================================================

import { normalizeRoomCode } from './roomcode.js';

export const PROXIMITY_PROOF_VERSION = 1;
export const DEFAULT_FRAME_MS = 360;
const PREAMBLE_BITS = Object.freeze([1, 0, 1, 1, 0, 1, 0, 0]);
const DEFAULT_DATA_BITS = 24;
const DEFAULT_MIN_CONFIDENCE = 0.9;

const enc = () => new TextEncoder();

async function digest(value) {
  const bytes = enc().encode(String(value || ''));
  const subtle = globalThis.crypto?.subtle;
  if (subtle?.digest) return new Uint8Array(await subtle.digest('SHA-256', bytes));
  // FNV-1a-derived 32-byte fallback (deterministic; not cryptographic).
  const out = new Uint8Array(32);
  let seed = 0x811c9dc5;
  for (let round = 0; round < 8; round++) {
    let h = seed ^ (round * 0x9e3779b1);
    bytes.forEach((b) => {
      h ^= b;
      h = Math.imul(h, 0x01000193) >>> 0;
    });
    out[round * 4] = (h >>> 24) & 0xff;
    out[round * 4 + 1] = (h >>> 16) & 0xff;
    out[round * 4 + 2] = (h >>> 8) & 0xff;
    out[round * 4 + 3] = h & 0xff;
    seed = h;
  }
  return out;
}

const bytesToBits = (bytes) => {
  const bits = [];
  bytes.forEach((b) => {
    for (let o = 7; o >= 0; o--) bits.push((b >> o) & 1);
  });
  return bits;
};
const byteToBits = (b) => {
  const bits = [];
  for (let o = 7; o >= 0; o--) bits.push((b >> o) & 1);
  return bits;
};
const checksumBits = (bits) => bits.reduce((c, bit, i) => (c ^ ((bit ? 0xa5 : 0x3c) + i * 17)) & 0xff, 0);
const hex = (bytes) => Array.from(bytes).map((b) => b.toString(16).padStart(2, '0')).join('');

/** Coerce observed input ('1011…' | number[] | {on}[]) to a 0/1 array. */
export function normalizeBits(value) {
  if (typeof value === 'string') return Array.from(value).filter((c) => c === '0' || c === '1').map(Number);
  if (!Array.isArray(value)) return [];
  return value
    .map((bit) => (typeof bit === 'object' && bit ? (bit.on ? 1 : 0) : bit === true || Number(bit) === 1 ? 1 : 0))
    .filter((bit) => bit === 0 || bit === 1);
}

// Slide the expected sequence across the observed stream; best alignment wins.
function bestMatch(expected, observed) {
  if (!expected.length || observed.length < expected.length) return { offset: -1, matches: 0, confidence: 0 };
  let best = { offset: 0, matches: -1, confidence: 0 };
  for (let offset = 0; offset <= observed.length - expected.length; offset++) {
    let matches = 0;
    for (let i = 0; i < expected.length; i++) if (expected[i] === observed[offset + i]) matches++;
    if (matches > best.matches) best = { offset, matches, confidence: matches / expected.length };
  }
  return best;
}

const seedOf = (roomCode, token) => [`waves.proximity.v${PROXIMITY_PROOF_VERSION}`, normalizeRoomCode(roomCode), String(token || '')].join('\n');

/**
 * Build the pattern the DISPLAYING device flashes.
 * @param {{roomCode:string, token:string, frameMs?:number, dataBits?:number}} opts
 * @returns {Promise<{roomCode,frameMs,bits,patternToken,checksum,fingerprint,shortCode,frames}>}
 */
export async function createProximityProof({ roomCode, token, frameMs = DEFAULT_FRAME_MS, dataBits = DEFAULT_DATA_BITS } = {}) {
  const code = normalizeRoomCode(roomCode);
  if (code.length !== 6) throw new Error('proximity proof requires a 6-character room code');
  if (!token) throw new Error('proximity proof requires a session token');

  const d = await digest(seedOf(code, token));
  const payloadBits = bytesToBits(d).slice(0, dataBits);
  const checksum = checksumBits(payloadBits);
  const bits = [...PREAMBLE_BITS, ...payloadBits, ...byteToBits(checksum)];
  const normFrameMs = Math.max(300, Number(frameMs) || 0);

  return Object.freeze({
    v: PROXIMITY_PROOF_VERSION,
    roomCode: code,
    frameMs: normFrameMs,
    dataBits: payloadBits.length,
    checksum,
    fingerprint: hex(d.slice(0, 8)),
    shortCode: `${code}-${hex(d.slice(0, 2)).toUpperCase()}`,
    patternToken: bits.join(''),
    bits: Object.freeze(bits),
    frames: Object.freeze(bits.map((bit, index) => Object.freeze({ index, bit, on: bit === 1, durationMs: normFrameMs }))),
  });
}

/**
 * Verify what the OBSERVING device saw. Recomputes the expected pattern from the
 * shared secret and scores the best alignment against the observed flashes.
 * @returns {Promise<{ok:boolean, reason:string, confidence:number, matchedBits:number, totalBits:number, offset:number}>}
 */
export async function verifyProximityProof({ roomCode, token, observedBits, minConfidence = DEFAULT_MIN_CONFIDENCE } = {}) {
  const expected = await createProximityProof({ roomCode, token });
  const observed = normalizeBits(observedBits);
  const match = bestMatch(expected.bits, observed);
  const ok = match.confidence >= Number(minConfidence || DEFAULT_MIN_CONFIDENCE);
  return Object.freeze({
    ok,
    reason: ok ? 'verified' : 'insufficient-pattern-match',
    confidence: match.confidence,
    matchedBits: match.matches,
    totalBits: expected.bits.length,
    offset: match.offset,
  });
}
