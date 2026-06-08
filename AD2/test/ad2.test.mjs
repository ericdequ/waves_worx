// =============================================================================
// waves_worx/AD2 — tests. Pure, no hardware: the mock device synthesizes the
// samples a real AD2 wire-loopback would capture, and we prove the full
// encode → play/capture → sense → decode loop round-trips.
// =============================================================================
import assert from 'node:assert/strict';
import test from 'node:test';

import { decode, encode } from '../../src/modem.js';
import {
  createMockDevice,
  detectTone,
  samplesToFreqs,
  senseFeatures,
  symbolFreqs,
  synthSamples,
} from '../src/index.js';

const SR = 48000;

test('Goertzel picks the right tone out of a pure sine window', () => {
  const freqs = symbolFreqs();
  const target = freqs[10]; // symbol 10
  const samples = synthSamples([{ freqHz: target, durationMs: 60 }], { sampleRate: SR });
  const hit = detectTone(samples, freqs, SR);
  assert.equal(hit.freqHz, target);
  assert.equal(hit.symbol, 10);
});

test('full loop: encode → mock AD2 capture → samplesToFreqs → decode round-trips', async () => {
  const message = 'meet at 9';
  const tones = encode(message);
  const dev = createMockDevice({ sampleRate: SR });

  const samples = await dev.playAndCapture(tones);
  const freqs = samplesToFreqs(samples, { sampleRate: SR });
  const out = decode(freqs);

  assert.equal(out.ok, true, out.reason);
  assert.equal(Buffer.from(out.bytes).toString('utf8'), message);
});

test('decode survives a noisy analog channel', async () => {
  const message = 'hi ric';
  const tones = encode(message);
  const dev = createMockDevice({ sampleRate: SR, noise: 0.25, seed: 7 });

  const samples = await dev.playAndCapture(tones);
  const out = decode(samplesToFreqs(samples, { sampleRate: SR }));

  assert.equal(out.ok, true, out.reason);
  assert.equal(Buffer.from(out.bytes).toString('utf8'), message);
});

test('senseFeatures grounds a captured tone into a feature map (the Pidar mirror)', () => {
  const freqs = symbolFreqs();
  const target = freqs[12];
  const samples = synthSamples([{ freqHz: target, durationMs: 120 }], { sampleRate: SR });

  const f = senseFeatures(samples, { sampleRate: SR });
  assert.equal(f.peakSymbol, 12);
  assert.equal(f.peakHz, target);
  assert.ok(f.rms > 0.5 && f.rms < 1.0, `rms ${f.rms}`); // ~0.707 for a unit sine
  assert.ok(Math.abs(f.centroidHz - target) < 80, `centroid ${f.centroidHz} vs ${target}`);
  assert.equal(f.bandEnergies.length, 16);
});

test('silence senses as low energy, no dominant symbol energy', () => {
  const samples = new Float64Array(SR / 10); // 0.1s of zeros
  const f = senseFeatures(samples, { sampleRate: SR });
  assert.equal(f.rms, 0);
  assert.equal(f.bandEnergies.reduce((a, b) => a + b, 0), 0);
});
