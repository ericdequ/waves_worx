#!/usr/bin/env node
// =============================================================================
// waves_worx/AD2 — bench: the end-to-end demo orchestrator.
//
// Ties the pure JS codec to a device (real AD2 via python/ad2_driver.py, or the
// pure mock) and runs the whole loop: a message becomes tones, the device plays
// + captures them over a wire loopback, Goertzel recovers the tones, `sense`
// reports the Pidar-style feature map, and `decode` reads the message back.
//
//   node bench.mjs --mock "meet at 9"     # no hardware — pure synthesis
//   node bench.mjs "meet at 9"            # real AD2 (W1 → 1+), needs WaveForms
//   node bench.mjs --mock --noise 0.3 hi  # model a dirtier analog channel
// =============================================================================
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { decode, encode } from '../src/modem.js';
import { createMockDevice, samplesToFreqs, senseFeatures } from './src/index.js';

const here = dirname(fileURLToPath(import.meta.url));
const argv = process.argv.slice(2);
const flag = (name) => argv.includes(`--${name}`);
const opt = (name, def) => {
  const i = argv.indexOf(`--${name}`);
  return i >= 0 && argv[i + 1] ? argv[i + 1] : def;
};
const message = argv.filter((a, i) => !a.startsWith('--') && argv[i - 1] !== '--noise').join(' ') || 'meet at 9';

const SAMPLE_RATE = Number(opt('rate', 48000));

/** Capture via the real AD2 by shelling out to the python driver. */
function captureViaAD2(tones) {
  return new Promise((resolve, reject) => {
    const py = spawn('python3', [join(here, 'python', 'ad2_driver.py')]);
    let out = '';
    let err = '';
    py.stdout.on('data', (d) => (out += d));
    py.stderr.on('data', (d) => (err += d));
    py.on('error', reject);
    py.on('close', (code) => {
      if (code !== 0) return reject(new Error(err.trim() || `ad2_driver exited ${code}`));
      try {
        resolve(JSON.parse(out).samples);
      } catch (e) {
        reject(new Error(`bad driver output: ${e.message}`));
      }
    });
    py.stdin.write(JSON.stringify({ tones, sampleRate: SAMPLE_RATE }));
    py.stdin.end();
  });
}

const tones = encode(message);
const device = flag('mock')
  ? createMockDevice({ sampleRate: SAMPLE_RATE, noise: Number(opt('noise', 0)) })
  : null;

const samples = device ? await device.playAndCapture(tones) : await captureViaAD2(tones);
const features = senseFeatures(samples, { sampleRate: SAMPLE_RATE });
const result = decode(samplesToFreqs(samples, { sampleRate: SAMPLE_RATE }));

console.log(`message   : ${JSON.stringify(message)}`);
console.log(`device    : ${device ? device.kind : 'analog-discovery-2 (W1 → 1+)'}`);
console.log(`tones      : ${tones.length} symbols, ${samples.length} samples @ ${SAMPLE_RATE} Hz`);
console.log(`sense      : peak ${features.peakHz} Hz (symbol ${features.peakSymbol}), ` +
  `centroid ${features.centroidHz.toFixed(0)} Hz, rms ${features.rms.toFixed(3)}`);
console.log(`decoded    : ${result.ok ? JSON.stringify(Buffer.from(result.bytes).toString('utf8')) : `✗ ${result.reason}`}`);
process.exit(result.ok ? 0 : 1);
