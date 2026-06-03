// =============================================================================
// waves_worx — channel: one interface over every physical/local transport
// =============================================================================
// A Channel moves frames between co-present devices with NO cloud in the path:
// Bluetooth LE, ultrasonic sound, light/screen flashes, QR hand-off, Wi-Fi LAN,
// WebRTC. The lab codes against this interface so the same chunked transfer
// (./chunk.js) runs over whichever channel is available.
//
// CHANNEL_KINDS records each channel's physical properties (range, bandwidth,
// duplex, line-of-sight, needs-native) so a ladder can pick the best available
// one. A loopback channel is provided for tests and local demos.
// Generalized from BEV's bevnet transport adapters.
// =============================================================================

import { cloneBytes } from './frame.js';

/** Physical properties of each channel kind (qualitative, for ladder choice). */
export const CHANNEL_KINDS = Object.freeze({
  loopback: { rangeM: 0, bandwidth: 'high', duplex: true, lineOfSight: false, native: false },
  qr: { rangeM: 1, bandwidth: 'low', duplex: false, lineOfSight: true, native: false },
  light: { rangeM: 3, bandwidth: 'low', duplex: false, lineOfSight: true, native: false },
  sound: { rangeM: 8, bandwidth: 'low', duplex: true, lineOfSight: false, native: true },
  ble: { rangeM: 30, bandwidth: 'medium', duplex: true, lineOfSight: false, native: true },
  wifiLan: { rangeM: 60, bandwidth: 'high', duplex: true, lineOfSight: false, native: false },
  webrtc: { rangeM: Infinity, bandwidth: 'high', duplex: true, lineOfSight: false, native: false },
});

const CHANNEL_METHODS = ['send', 'subscribe'];

/** Throw unless `channel` satisfies the Channel contract. */
export function assertChannel(channel) {
  if (!channel || !CHANNEL_KINDS[channel.kind]) {
    throw new TypeError('waves_worx: channel requires a known kind');
  }
  for (const m of CHANNEL_METHODS) {
    if (typeof channel[m] !== 'function') {
      throw new TypeError(`waves_worx: channel.${m}() is required`);
    }
  }
  return channel;
}

/**
 * Given the set of currently-available channel kinds, pick the best by a
 * preference ladder (default: fastest/longest-range first that's available).
 * @param {string[]} available
 * @param {string[]} [ladder]
 * @returns {string|null}
 */
export function pickChannel(available, ladder = ['webrtc', 'wifiLan', 'ble', 'sound', 'light', 'qr', 'loopback']) {
  const set = new Set(available);
  return ladder.find((kind) => set.has(kind)) ?? null;
}

/**
 * In-memory bus that delivers frames to every subscriber — models a shared
 * physical medium (the "air" two devices share). Used by loopback channels for
 * tests and single-process demos.
 */
export function createLoopbackBus() {
  const subs = new Set();
  return {
    publish(fromId, bytes, meta = {}) {
      const frame = cloneBytes(bytes);
      let delivered = 0;
      for (const s of subs) {
        if (!s.echoSelf && s.id === fromId) continue;
        delivered++;
        s.cb(new Uint8Array(frame), { ...meta, fromId });
      }
      return delivered;
    },
    subscribe(id, cb, { echoSelf = false } = {}) {
      const entry = { id, cb, echoSelf };
      subs.add(entry);
      return () => subs.delete(entry);
    },
    get size() {
      return subs.size;
    },
  };
}

/**
 * A Channel backed by a loopback bus. Multiple loopback channels sharing a bus
 * behave like devices in radio range of each other.
 *
 * @param {{id?:string, bus?:ReturnType<createLoopbackBus>, echoSelf?:boolean}} [opts]
 */
export function createLoopbackChannel({ id = 'device', bus = createLoopbackBus(), echoSelf = false } = {}) {
  let sent = 0;
  let received = 0;
  return assertChannel({
    kind: 'loopback',
    id,
    bus,
    send(bytes, meta) {
      sent++;
      return bus.publish(id, bytes, meta);
    },
    subscribe(cb) {
      return bus.subscribe(
        id,
        (bytes, meta) => {
          received++;
          cb(bytes, meta);
        },
        { echoSelf },
      );
    },
    diagnostics() {
      return { kind: 'loopback', id, sent, received, peers: bus.size };
    },
  });
}
