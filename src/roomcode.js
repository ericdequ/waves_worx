// =============================================================================
// waves_worx — room code: a short, human-shareable session id
// =============================================================================
// Six characters, uppercase, ambiguity-pruned alphabet (no O/0/I/1 confusion).
// Spoken aloud, typed, or flashed — the entry ticket to a no-cloud session.
// =============================================================================

// Crockford-ish: drop visually ambiguous glyphs.
const ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
export const ROOM_CODE_LENGTH = 6;

/** Normalize user input to a canonical room code (uppercase, alnum, 6 max). */
export const normalizeRoomCode = (value) =>
  String(value || '')
    .trim()
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, ROOM_CODE_LENGTH);

/**
 * Generate a room code. `rand` is injectable (default crypto) so callers can
 * seed deterministically in tests.
 * @param {() => number} [rand]  Returns a float in [0,1).
 */
export function generateRoomCode(rand = secureUnit) {
  let out = '';
  for (let i = 0; i < ROOM_CODE_LENGTH; i++) {
    out += ALPHABET[Math.floor(rand() * ALPHABET.length)];
  }
  return out;
}

/** True for a structurally-valid (6-char) code. */
export const isRoomCode = (value) => normalizeRoomCode(value).length === ROOM_CODE_LENGTH;

// Cryptographically-strong unit float when available; Math.random fallback is
// fine for a non-secret session id (the proof, not the code, gates access).
function secureUnit() {
  const c = globalThis.crypto;
  if (c?.getRandomValues) {
    const buf = new Uint32Array(1);
    c.getRandomValues(buf);
    return buf[0] / 2 ** 32;
  }
  return Math.random();
}
