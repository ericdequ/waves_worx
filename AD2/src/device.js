// =============================================================================
// waves_worx/AD2 — device: the hardware adapter contract + a pure mock
// =============================================================================
// Every AD2 backend implements ONE method — play a tone list out of the AWG and
// capture the resulting samples on the scope (a wire loopback: W1 → 1+). That's
// the whole physical hop. By defining it as a contract, the same codec runs
// over real hardware (python/ad2_driver.py) OR over `createMockDevice()`, which
// SYNTHESIZES the samples in pure JS — so the full sense→decode loop is testable
// with no AD2 attached (the sibling of waves_worx's createLoopbackChannel).
//
//   AD2_DEVICE := { kind, sampleRate, playAndCapture(tones) → samples, close() }
//
// PURE (the mock). No hardware, no deps.
// =============================================================================

/** Deterministic LCG so test "noise" is reproducible (never Math.random in a test). */
const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    return s / 0xffffffff;
  };
};

/**
 * Synthesize the voltage samples a wire loopback would produce for a tone list —
 * the math the AD2's AWG+ADC do physically. Concatenated sine bursts, one per
 * tone, each `durationMs` long, with optional uniform noise to model a real
 * (slightly dirty) analog channel.
 * @param {Array<{freqHz:number,durationMs:number}>} tones
 * @param {{ sampleRate:number, amplitude?:number, noise?:number, seed?:number }} opts
 * @returns {Float64Array}
 */
export function synthSamples(tones, { sampleRate, amplitude = 1, noise = 0, seed = 1 } = {}) {
  const rand = lcg(seed);
  const perTone = tones.map((t) => Math.round((sampleRate * t.durationMs) / 1000));
  const total = perTone.reduce((a, b) => a + b, 0);
  const out = new Float64Array(total);
  let idx = 0;
  tones.forEach((tone, ti) => {
    const n = perTone[ti];
    const w = (2 * Math.PI * tone.freqHz) / sampleRate;
    for (let i = 0; i < n; i++) {
      const noiseV = noise ? (rand() * 2 - 1) * noise : 0;
      out[idx++] = amplitude * Math.sin(w * i) + noiseV;
    }
  });
  return out;
}

/**
 * A pure, no-hardware AD2 stand-in modelling a clean W1→1+ wire loopback.
 * @param {{ sampleRate?:number, amplitude?:number, noise?:number, seed?:number }} [opts]
 */
export function createMockDevice({ sampleRate = 48000, amplitude = 1, noise = 0, seed = 1 } = {}) {
  return Object.freeze({
    kind: 'mock-ad2',
    sampleRate,
    async playAndCapture(tones) {
      return synthSamples(tones, { sampleRate, amplitude, noise, seed });
    },
    async close() {},
  });
}
