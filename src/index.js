// =============================================================================
// waves_worx — R&D lab for no-cloud, physical-layer data transport + the
// "games arcade" that demonstrates physical security in a hands-on way.
//
// For AUTHORIZED security testing, teaching, CTF-style demos, and privacy-
// preserving local data transfer between consenting, co-present devices. Every
// primitive here keeps data in the room — there is no cloud in the path.
// =============================================================================

// frame — the universal byte layer
export { toBytes, cloneBytes, encodeJSON, decodeJSON, bytesToBase64, base64ToBytes, bytesToBase64Url, base64UrlToBytes, crc32 } from './frame.js';

// chunk — send a payload over a low-bandwidth, lossy channel
export { chunk, Reassembler, DEFAULT_CHUNK_BYTES } from './chunk.js';

// channel — one interface over every physical/local transport
export { CHANNEL_KINDS, assertChannel, pickChannel, createLoopbackBus, createLoopbackChannel } from './channel.js';

// modem — send bytes over sound (FSK codec)
export { encode, decode, modemBand, DEFAULT_MODEM } from './modem.js';

// arcade — consensual proximity primitives
export { normalizeRoomCode, generateRoomCode, isRoomCode, ROOM_CODE_LENGTH } from './roomcode.js';
export { createProximityProof, verifyProximityProof, normalizeBits, PROXIMITY_PROOF_VERSION, DEFAULT_FRAME_MS } from './proximity.js';
export { createPairing, PAIRING_STATE, PAIRING_VERSION } from './pairing.js';
