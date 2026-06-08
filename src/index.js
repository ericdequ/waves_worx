// =============================================================================
// waves_worx — R&D lab for no-cloud, physical-layer data transport + the
// "games arcade" that demonstrates physical security in a hands-on way.
//
// For AUTHORIZED security testing, teaching, CTF-style demos, and privacy-
// preserving local data transfer between consenting, co-present devices. Every
// primitive here keeps data in the room — there is no cloud in the path.
// =============================================================================

// frame — the universal byte layer
export { base64ToBytes, base64UrlToBytes, bytesToBase64, bytesToBase64Url, cloneBytes, crc32,decodeJSON, encodeJSON, toBytes } from './frame.js';

// chunk — send a payload over a low-bandwidth, lossy channel
export { chunk, DEFAULT_CHUNK_BYTES,Reassembler } from './chunk.js';

// channel — one interface over every physical/local transport
export { assertChannel, CHANNEL_KINDS, createLoopbackBus, createLoopbackChannel,pickChannel } from './channel.js';

// BLE game payloads — 21-byte detect/decode frames for manufacturer data tests
export {
  BLE_GAME_FRAME_LEN,
  BLE_GAME_KIND,
  BLE_GAME_MAGIC,
  BLE_GAME_PAYLOAD_BYTES,
  BLE_GAME_VERSION,
  decodeBleGameTransfer,
  detectBleGameFrame,
  encodeBleGameFrame,
  encodeBleGameTransfer,
} from './blegame.js';

// modem — send bytes over sound (FSK codec)
export { decode, DEFAULT_MODEM,encode, modemBand } from './modem.js';

// arcade — consensual proximity primitives
export { createPairing, PAIRING_STATE, PAIRING_VERSION } from './pairing.js';
export { createProximityProof, DEFAULT_FRAME_MS,normalizeBits, PROXIMITY_PROOF_VERSION, verifyProximityProof } from './proximity.js';
export { generateRoomCode, isRoomCode, normalizeRoomCode, ROOM_CODE_LENGTH } from './roomcode.js';
