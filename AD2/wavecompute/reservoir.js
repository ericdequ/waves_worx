// =============================================================================
// AD2/wavecompute — reservoir: reservoir computing (Echo State Network)
// =============================================================================
// A fixed, random, recurrent NONLINEAR medium — the "reservoir" — that you do
// NOT train. You only train a cheap LINEAR readout on top. The reservoir's rich
// dynamics project a temporal input into a high-dimensional space where time-
// dependent patterns become linearly separable. Physically the reservoir can be
// any nonlinear dynamical medium (a bucket of water, a photonic loop, an analog
// circuit) — its waves do the hard part for free; only the readout learns.
//
//   s(t) = tanh( W·s(t-1) + Win·u(t) )     fixed random W, Win  (the medium)
//   y(t) = Wout · [1; s(t)]                trained linear readout (ridge)
//
// Quantum cousin: quantum reservoir computing. Pure, zero-dep, node-safe.
// =============================================================================

const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32;
};

/** Standard-normal sample via Box–Muller (deterministic given the rng). */
const randn = (rnd) => {
  let u = 0;
  let v = 0;
  while (u === 0) u = rnd();
  while (v === 0) v = rnd();
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
};

const matVec = (M, x) =>
  M.map((row) => {
    let s = 0;
    for (let j = 0; j < x.length; j++) s += row[j] * x[j];
    return s;
  });

/** Dot product. */
export const dot = (w, x) => {
  let s = 0;
  for (let i = 0; i < w.length; i++) s += w[i] * x[i];
  return s;
};

/**
 * Build a fixed random reservoir. Recurrent weights W are rescaled to the target
 * spectral radius (< 1 → the "echo state property": memory that fades).
 */
export function createReservoir({ nIn = 1, nRes = 60, spectralRadius = 0.9, inputScale = 1, seed = 1 } = {}) {
  const rnd = lcg(seed);
  const W = Array.from({ length: nRes }, () => Array.from({ length: nRes }, () => randn(rnd)));
  const Win = Array.from({ length: nRes }, () => Array.from({ length: nIn }, () => (rnd() * 2 - 1) * inputScale));

  // Estimate the spectral radius by power iteration, then rescale W to target.
  let v = Array.from({ length: nRes }, () => rnd());
  let norm = Math.hypot(...v) || 1;
  v = v.map((x) => x / norm);
  let rho = 1;
  for (let it = 0; it < 60; it++) {
    const Wv = matVec(W, v);
    rho = Math.hypot(...Wv) || 1;
    v = Wv.map((x) => x / rho);
  }
  const scale = spectralRadius / (rho || 1);
  for (let i = 0; i < nRes; i++) for (let j = 0; j < nRes; j++) W[i][j] *= scale;

  return { W, Win, nRes, nIn };
}

/** Run an input sequence through the reservoir; return the state at each step. */
export function runStream(res, inputs) {
  const states = [];
  let s = new Float64Array(res.nRes);
  for (const u of inputs) {
    const next = new Float64Array(res.nRes);
    for (let i = 0; i < res.nRes; i++) {
      let acc = 0;
      const Wi = res.W[i];
      for (let j = 0; j < res.nRes; j++) acc += Wi[j] * s[j];
      const Wini = res.Win[i];
      for (let k = 0; k < res.nIn; k++) acc += Wini[k] * u[k];
      next[i] = Math.tanh(acc);
    }
    s = next;
    states.push(s);
  }
  return states;
}

/** Solve A·x = b by Gauss-Jordan with partial pivoting. */
function solve(A, b) {
  const n = b.length;
  const M = A.map((row, i) => [...row, b[i]]);
  for (let col = 0; col < n; col++) {
    let piv = col;
    for (let r = col + 1; r < n; r++) if (Math.abs(M[r][col]) > Math.abs(M[piv][col])) piv = r;
    [M[col], M[piv]] = [M[piv], M[col]];
    const d = M[col][col] || 1e-12;
    for (let r = 0; r < n; r++) {
      if (r === col) continue;
      const f = M[r][col] / d;
      for (let c = col; c <= n; c++) M[r][c] -= f * M[col][c];
    }
  }
  const x = new Float64Array(n);
  for (let i = 0; i < n; i++) x[i] = M[i][n] / (M[i][i] || 1e-12);
  return x;
}

/**
 * Ridge regression: fit w minimizing ‖Xw − y‖² + λ‖w‖². X is rows of feature
 * vectors, y is scalar targets. The ONLY trained part of the whole system.
 */
export function ridgeFit(X, y, lambda = 1e-3) {
  const F = X[0].length;
  const A = Array.from({ length: F }, () => new Float64Array(F));
  const b = new Float64Array(F);
  for (let t = 0; t < X.length; t++) {
    const xt = X[t];
    for (let i = 0; i < F; i++) {
      b[i] += xt[i] * y[t];
      for (let j = 0; j < F; j++) A[i][j] += xt[i] * xt[j];
    }
  }
  for (let i = 0; i < F; i++) A[i][i] += lambda;
  return solve(A, b);
}

/** Feature vector for a reservoir state: [bias, ...state]. */
export const stateFeature = (s) => [1, ...s];
