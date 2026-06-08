// =============================================================================
// waves_worx/AD2 — dsp: turn raw ADC samples back into tones (the pure core)
// =============================================================================
// The Analog Discovery 2 hands us a buffer of voltage samples. To recover the
// data the `modem` encoded, we must answer one question per symbol window:
// "which of the 16 FSK tones is present here?" The Goertzel algorithm answers
// exactly that — it's a single-bin DFT that measures the power at ONE target
// frequency in O(n), far cheaper than a full FFT when you only care about the
// 16 candidate tones. This is the same trick a real radio's tone detector uses.
//
// PURE — no hardware, no deps. Samples in (Float64Array | number[]), tones out.
// The AD2 (or a microphone, or the mock device) is a thin adapter that produces
// the samples; the recovery math lives here so it's fully unit-testable.
// =============================================================================

import { DEFAULT_MODEM } from '../../src/modem.js';

const SYMBOLS = 16; // mirrors modem.js — one nibble (16 tones) per symbol

/** The 16 FSK tone frequencies (Hz) a modem config occupies, symbol 0..15. */
export const symbolFreqs = (config = {}) => {
  const cfg = { ...DEFAULT_MODEM, ...config };
  return Array.from({ length: SYMBOLS }, (_, sym) => cfg.baseFreq + sym * cfg.stepFreq);
};

/**
 * Goertzel power of `freqHz` over `samples` taken at `sampleRate` Hz.
 * Returns a window-length-normalized magnitude² (relative, unit-agnostic) so
 * windows of different sizes stay comparable.
 * @param {ArrayLike<number>} samples
 * @param {number} freqHz
 * @param {number} sampleRate
 * @returns {number}
 */
export function goertzelPower(samples, freqHz, sampleRate) {
  const n = samples.length;
  if (n === 0 || !(sampleRate > 0)) return 0;
  const omega = (2 * Math.PI * freqHz) / sampleRate;
  const coeff = 2 * Math.cos(omega);
  let s1 = 0;
  let s2 = 0;
  for (let i = 0; i < n; i++) {
    const s0 = samples[i] + coeff * s1 - s2;
    s2 = s1;
    s1 = s0;
  }
  const power = s1 * s1 + s2 * s2 - coeff * s1 * s2;
  return power / (n * n);
}

/**
 * The strongest of a set of candidate tones in one window.
 * @param {ArrayLike<number>} samples
 * @param {number[]} candidateFreqs
 * @param {number} sampleRate
 * @returns {{ freqHz:number, power:number, symbol:number }}
 */
export function detectTone(samples, candidateFreqs, sampleRate) {
  let best = { freqHz: 0, power: -Infinity, symbol: -1 };
  candidateFreqs.forEach((freqHz, symbol) => {
    const power = goertzelPower(samples, freqHz, sampleRate);
    if (power > best.power) best = { freqHz, power, symbol };
  });
  return best;
}

const windowOf = (samples, start, length) =>
  typeof samples.subarray === 'function'
    ? samples.subarray(start, start + length)
    : samples.slice(start, start + length);

/**
 * Slice a captured buffer into per-symbol windows and report the tone in each —
 * the bridge from AD2 samples to `modem.decode()`'s expected frequency list.
 * @param {ArrayLike<number>} samples
 * @param {{ sampleRate:number, symbolMs?:number, config?:object }} opts
 * @returns {number[]} one frequency (Hz) per symbol window, in order
 */
export function samplesToFreqs(samples, { sampleRate, symbolMs, config = {} } = {}) {
  const cfg = { ...DEFAULT_MODEM, ...config };
  const freqs = symbolFreqs(cfg);
  const perSymbol = Math.round((sampleRate * (symbolMs ?? cfg.symbolMs)) / 1000);
  if (!(perSymbol > 0)) return [];
  const out = [];
  for (let start = 0; start + perSymbol <= samples.length; start += perSymbol) {
    out.push(detectTone(windowOf(samples, start, perSymbol), freqs, sampleRate).freqHz);
  }
  return out;
}
