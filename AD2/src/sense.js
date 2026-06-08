// =============================================================================
// waves_worx/AD2 — sense: a captured signal → a compact feature map (Pidar)
// =============================================================================
// This is the bridge to BEV's "Pidar sensing" idea and to how Ric/EPU processes
// data. Pidar = robot Ric, checked-in here-now, turning what it SENSES into a
// FEATURE MAP it can ground and remember. The AD2 makes that literal: continuous
// physical voltage → discrete samples → a small, fixed feature vector.
//
//   physical signal ──sense──▶ { rms, peakHz, peakSymbol, centroidHz, bandEnergies }
//
// The parallel to EPU grounding is exact and intentional:
//   • peakSymbol (0..15)  ≈ the discrete #symbol token (the "what")  → like #emoji
//   • centroidHz          ≈ a 1-D "vibe" scalar (spectral center of mass)
//   • bandEnergies[16]    ≈ the pre-vector feature map a later embedding refines
// Just as TST grounds in time+space+#type BEFORE running vectors, `senseFeatures`
// produces the cheap, classical, debuggable features first. Same philosophy as
// unicodeType.js: the classical key that also debugs the vectors above it.
//
// PURE — samples in, features out. No hardware, no deps.
// =============================================================================

import { goertzelPower, symbolFreqs } from './dsp.js';
import { DEFAULT_MODEM } from '../../src/modem.js';

/**
 * Reduce a captured sample buffer to a compact feature record.
 * @param {ArrayLike<number>} samples
 * @param {{ sampleRate:number, config?:object }} opts
 * @returns {Readonly<{
 *   rms:number, peakHz:number, peakSymbol:number, centroidHz:number,
 *   bandEnergies:ReadonlyArray<number>, sampleRate:number, sampleCount:number
 * }>}
 */
export function senseFeatures(samples, { sampleRate, config = {} } = {}) {
  const cfg = { ...DEFAULT_MODEM, ...config };
  const freqs = symbolFreqs(cfg);
  const n = samples.length;

  // RMS — overall "energy"/loudness of what we sensed.
  let sumSq = 0;
  for (let i = 0; i < n; i++) sumSq += samples[i] * samples[i];
  const rms = n ? Math.sqrt(sumSq / n) : 0;

  // Band energies across the 16 modem tones — the feature map.
  const bandEnergies = freqs.map((f) => goertzelPower(samples, f, sampleRate));

  // Peak tone (the sensed #symbol) + spectral centroid (a scalar "vibe").
  let peakSymbol = 0;
  for (let i = 1; i < bandEnergies.length; i++) {
    if (bandEnergies[i] > bandEnergies[peakSymbol]) peakSymbol = i;
  }
  const totalE = bandEnergies.reduce((a, b) => a + b, 0) || 1;
  const centroidHz = freqs.reduce((acc, f, i) => acc + f * (bandEnergies[i] / totalE), 0);

  return Object.freeze({
    rms,
    peakHz: freqs[peakSymbol],
    peakSymbol,
    centroidHz,
    bandEnergies: Object.freeze(bandEnergies),
    sampleRate,
    sampleCount: n,
  });
}
