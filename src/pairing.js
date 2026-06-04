// =============================================================================
// waves_worx — pairing: establish a session between two co-present devices
// =============================================================================
// A tiny three-message handshake (offer → answer → confirm) that two devices
// run over ANY channel from ./channel.js to agree on a shared session, bound to
// a room code. It is transport-agnostic and side-effect-free: each side feeds
// received messages in and gets the next message to send out, plus a state.
//
// The session id is derived from both nonces + the room code, so a passive
// eavesdropper who only saw the room code cannot forge it, and the two sides
// independently compute the same id (proof they completed the exchange). Pair
// this with the optical proximity proof (./proximity.js) when you also need to
// prove physical co-location, not just message exchange.
// =============================================================================

import { crc32 } from './frame.js';
import { normalizeRoomCode } from './roomcode.js';

export const PAIRING_VERSION = 1;
export const PAIRING_STATE = Object.freeze({
  IDLE: 'idle',
  OFFERED: 'offered', // initiator sent offer, awaiting answer
  ANSWERED: 'answered', // responder sent answer, awaiting confirm
  PAIRED: 'paired',
  FAILED: 'failed',
});

const MSG = Object.freeze({ OFFER: 'offer', ANSWER: 'answer', CONFIRM: 'confirm' });

// Deterministic session id from room code + both nonces (order-independent so
// both sides compute the same value). Not a substitute for real key exchange —
// it's a co-completion token for consensual local pairing.
function deriveSession(roomCode, a, b) {
  const [x, y] = [String(a), String(b)].sort();
  const seed = `${normalizeRoomCode(roomCode)}|${x}|${y}`;
  // 64-bit-ish id from two CRC passes over salted seeds.
  const hi = crc32(`s1:${seed}`).toString(16).padStart(8, '0');
  const lo = crc32(`s2:${seed}`).toString(16).padStart(8, '0');
  return `${hi}${lo}`;
}

/**
 * Create a pairing endpoint.
 * @param {object} opts
 * @param {string} opts.roomCode
 * @param {'initiator'|'responder'} opts.role
 * @param {() => string} [opts.nonce]  injectable for tests
 */
export function createPairing({ roomCode, role, nonce = randomNonce }) {
  const code = normalizeRoomCode(roomCode);
  const myNonce = nonce();
  let state = PAIRING_STATE.IDLE;
  let theirNonce = null;
  let session = null;

  const fail = (reason) => {
    state = PAIRING_STATE.FAILED;
    return { state, reason, send: null, session: null };
  };
  const snapshot = (extra = {}) => ({ state, session, role, ...extra });

  return {
    get state() {
      return state;
    },
    get session() {
      return session;
    },

    /**
     * Produce the opening message. Initiator only; call once to begin.
     * @returns {{type:string, v:number, roomCode:string, nonce:string}|null}
     */
    start() {
      if (role !== 'initiator' || state !== PAIRING_STATE.IDLE) return null;
      state = PAIRING_STATE.OFFERED;
      return { type: MSG.OFFER, v: PAIRING_VERSION, roomCode: code, nonce: myNonce };
    },

    /**
     * Feed a received message. Returns { state, session, send } where `send`
     * is the next message to transmit (or null).
     */
    receive(msg) {
      if (!msg || msg.v !== PAIRING_VERSION) return snapshot({ send: null });
      if (normalizeRoomCode(msg.roomCode) !== code) return fail('room-code-mismatch');

      // Responder receives the offer → answers.
      if (msg.type === MSG.OFFER && role === 'responder' && state === PAIRING_STATE.IDLE) {
        theirNonce = msg.nonce;
        session = deriveSession(code, myNonce, theirNonce);
        state = PAIRING_STATE.ANSWERED;
        return snapshot({ send: { type: MSG.ANSWER, v: PAIRING_VERSION, roomCode: code, nonce: myNonce } });
      }

      // Initiator receives the answer → confirms + is paired.
      if (msg.type === MSG.ANSWER && role === 'initiator' && state === PAIRING_STATE.OFFERED) {
        theirNonce = msg.nonce;
        session = deriveSession(code, myNonce, theirNonce);
        state = PAIRING_STATE.PAIRED;
        return snapshot({ send: { type: MSG.CONFIRM, v: PAIRING_VERSION, roomCode: code, session } });
      }

      // Responder receives confirm → verifies both derived the same session.
      if (msg.type === MSG.CONFIRM && role === 'responder' && state === PAIRING_STATE.ANSWERED) {
        if (msg.session !== session) return fail('session-mismatch');
        state = PAIRING_STATE.PAIRED;
        return snapshot({ send: null });
      }

      return snapshot({ send: null });
    },
  };
}

function randomNonce() {
  const c = globalThis.crypto;
  if (c?.getRandomValues) {
    const buf = new Uint32Array(2);
    c.getRandomValues(buf);
    return `${buf[0].toString(16)}${buf[1].toString(16)}`;
  }
  return `${Math.floor(Math.random() * 2 ** 32).toString(16)}`;
}
