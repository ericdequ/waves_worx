// =============================================================================
// waves_worx — modem: send bytes over sound (or any frequency-shift channel)
// =============================================================================
// A frequency-shift-keying (FSK) codec. Each nibble (4 bits) maps to one of 16
// tones; a byte is two tones. A preamble syncs the receiver and a checksum byte
// guards integrity. The codec is PURE — it turns bytes into a list of
// { freqHz, durationMs } tones and back — so it's fully testable without audio
// hardware. The actual speaker/microphone is a thin adapter (see playTones /
// listenTones notes in the README); the hard part, the modulation, lives here.
//
// No cloud: two phones in the same room exchange data through the air.
// =============================================================================

import { toBytes, crc32 } from './frame.js';

const SYMBOLS = 16; // one nibble per tone

export const DEFAULT_MODEM = Object.freeze({
  baseFreq: 1200, // Hz of symbol 0
  stepFreq: 80, // Hz between adjacent symbols → 1200..2400 Hz band
  symbolMs: 60, // tone duration
  // Two preamble symbols outside the data progression, used to find the start.
  preamble: Object.freeze([15, 0, 15, 0]),
});

const freqForSymbol = (sym, cfg) => cfg.baseFreq + sym * cfg.stepFreq;
const symbolForFreq = (freq, cfg) => {
  const sym = Math.round((freq - cfg.baseFreq) / cfg.stepFreq);
  return Math.max(0, Math.min(SYMBOLS - 1, sym));
};

const bytesToSymbols = (bytes) => {
  const syms = [];
  for (const b of bytes) {
    syms.push((b >> 4) & 0x0f, b & 0x0f);
  }
  return syms;
};
const symbolsToBytes = (syms) => {
  const out = new Uint8Array(Math.floor(syms.length / 2));
  for (let i = 0; i < out.length; i++) out[i] = (syms[i * 2] << 4) | syms[i * 2 + 1];
  return out;
};

/**
 * Encode a payload into a tone list.
 * @param {Uint8Array|string} payload
 * @param {object} [config]  overrides for DEFAULT_MODEM
 * @returns {Array<{symbol:number, freqHz:number, durationMs:number}>}
 */
export function encode(payload, config = {}) {
  const cfg = { ...DEFAULT_MODEM, ...config };
  const bytes = toBytes(payload);
  const check = crc32(bytes) & 0xff; // 1-byte checksum keeps the tone count low
  const symbols = [...cfg.preamble, ...bytesToSymbols(bytes), ...bytesToSymbols(Uint8Array.from([check]))];
  return symbols.map((symbol) => ({ symbol, freqHz: freqForSymbol(symbol, cfg), durationMs: cfg.symbolMs }));
}

/**
 * Decode observed frequencies (Hz, one per detected tone — the receiver's
 * pitch tracker output) back into bytes. Tolerates frequency jitter via
 * nearest-symbol quantization. Finds the preamble to align, verifies checksum.
 * @param {number[]} observedFreqs
 * @param {object} [config]
 * @returns {{ ok:boolean, bytes:Uint8Array|null, reason:string }}
 */
export function decode(observedFreqs, config = {}) {
  const cfg = { ...DEFAULT_MODEM, ...config };
  const syms = observedFreqs.map((f) => symbolForFreq(f, cfg));

  // Locate the preamble.
  const pre = cfg.preamble;
  let start = -1;
  for (let i = 0; i + pre.length <= syms.length; i++) {
    let ok = true;
    for (let j = 0; j < pre.length; j++) if (syms[i + j] !== pre[j]) { ok = false; break; }
    if (ok) { start = i + pre.length; break; }
  }
  if (start < 0) return { ok: false, bytes: null, reason: 'no-preamble' };

  const dataSyms = syms.slice(start);
  if (dataSyms.length < 2 || dataSyms.length % 2 !== 0) return { ok: false, bytes: null, reason: 'truncated' };

  const all = symbolsToBytes(dataSyms);
  const payload = all.subarray(0, all.length - 1);
  const check = all[all.length - 1];
  if ((crc32(payload) & 0xff) !== check) return { ok: false, bytes: null, reason: 'checksum-mismatch' };

  return { ok: true, bytes: new Uint8Array(payload), reason: 'ok' };
}

/** The frequency band a config occupies, for picking a clear channel. */
export const modemBand = (config = {}) => {
  const cfg = { ...DEFAULT_MODEM, ...config };
  return { lowHz: cfg.baseFreq, highHz: freqForSymbol(SYMBOLS - 1, cfg) };
};
