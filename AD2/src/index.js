// =============================================================================
// waves_worx/AD2 — the Analog Discovery 2 bench backend for waves_worx.
//
// A real physical-layer backend for the pure waves_worx codecs: the AWG plays
// the modem's tones, the scope captures them, Goertzel recovers them, `sense`
// extracts a Pidar-style feature map. Pure DSP here; the hardware I/O is the
// thin adapter in python/ad2_driver.py. For AUTHORIZED teaching / bench R&D.
// =============================================================================

// dsp — samples ⇄ tones (the pure recovery core)
export { detectTone, goertzelPower, samplesToFreqs, symbolFreqs } from './dsp.js';

// device — the hardware-adapter contract + a pure, testable mock
export { createMockDevice, synthSamples } from './device.js';

// sense — captured signal → compact feature map (the Pidar mirror)
export { senseFeatures } from './sense.js';
