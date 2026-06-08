// =============================================================================
// AD2/wavecompute — ising: an oscillator Ising machine (Simulated Bifurcation)
// =============================================================================
// Wave dynamics solving a quantum-annealing-class problem, classically. Each
// variable is a nonlinear OSCILLATOR with a position x and momentum y. As a
// "pump" energy is ramped up, the network undergoes a bifurcation and each
// oscillator falls to x≈+1 or x≈−1 — and the configuration it settles into is
// the minimum-energy (ground) state of the Ising problem. This is Simulated
// Bifurcation (Goto et al.), the classical algorithm that competes with quantum
// annealers on combinatorial optimization (MAX-CUT, QUBO).
//
//   Ising energy:  E(s) = -½ · sᵀ J s        (s ∈ {-1,+1}ⁿ)
//   the machine minimizes E by letting the coupled oscillators settle.
//
// This is the honest "quantum-like speedup": the optimization is done by the
// physics of relaxation, not by enumerating states. Pure, zero-dep, node-safe.
// =============================================================================

const lcg = (seed) => {
  let s = seed >>> 0 || 1;
  return () => (s = (Math.imul(s, 1664525) + 1013904223) >>> 0) / 2 ** 32;
};

/** Ising energy E(s) = -½ sᵀJs for spins s ∈ {-1,+1}. */
export function isingEnergy(J, s) {
  const n = s.length;
  let e = 0;
  for (let i = 0; i < n; i++) {
    const Ji = J[i];
    for (let j = 0; j < n; j++) e += Ji[j] * s[i] * s[j];
  }
  return -0.5 * e;
}

/** Coupling scale c0 that keeps the bifurcation dynamics stable (~Goto 2019). */
function couplingScale(J) {
  const n = J.length;
  let sum = 0;
  let cnt = 0;
  for (let i = 0; i < n; i++) {
    for (let j = 0; j < n; j++) {
      if (i !== j) { sum += J[i][j] * J[i][j]; cnt += 1; }
    }
  }
  const sigma = Math.sqrt(sum / Math.max(1, cnt)) || 1;
  return 0.5 / (Math.sqrt(n) * sigma);
}

/** One ballistic-SB run: ramp the pump, let the oscillators bifurcate. */
function runSB(J, { steps, dt, seed }) {
  const n = J.length;
  const rnd = lcg(seed);
  const a0 = 1;
  const c0 = couplingScale(J);
  const x = new Float64Array(n);
  const y = new Float64Array(n);
  for (let i = 0; i < n; i++) {
    x[i] = (rnd() * 2 - 1) * 0.1;
    y[i] = (rnd() * 2 - 1) * 0.1;
  }
  for (let t = 0; t < steps; t++) {
    const a = a0 * (t / steps); // pump ramps 0 → a0 (the bifurcation drive)
    for (let i = 0; i < n; i++) {
      const Ji = J[i];
      let coup = 0;
      for (let j = 0; j < n; j++) coup += Ji[j] * x[j];
      y[i] += dt * (-(a0 - a) * x[i] + c0 * coup);
    }
    for (let i = 0; i < n; i++) {
      x[i] += dt * a0 * y[i];
      if (x[i] > 1) { x[i] = 1; y[i] = 0; } // inelastic walls — the "ballistic" SB
      else if (x[i] < -1) { x[i] = -1; y[i] = 0; }
    }
  }
  const s = new Int8Array(n);
  for (let i = 0; i < n; i++) s[i] = x[i] >= 0 ? 1 : -1;
  return s;
}

/**
 * Minimize an Ising energy by Simulated Bifurcation, taking the best of several
 * restarts (the machine is run a few times; lowest energy wins).
 * @param {number[][]} J  symmetric coupling matrix (zero diagonal)
 * @returns {{ spins: Int8Array, energy: number }}
 */
export function solveIsing(J, { steps = 800, dt = 0.25, restarts = 16, seed = 1 } = {}) {
  let best = null;
  let bestE = Infinity;
  for (let r = 0; r < restarts; r++) {
    const s = runSB(J, { steps, dt, seed: seed + r * 101 });
    const e = isingEnergy(J, s);
    if (e < bestE) { bestE = e; best = s; }
  }
  return { spins: best, energy: bestE };
}

/**
 * Solve MAX-CUT via the Ising machine. Edges are `[a, b]` or `[a, b, weight]`.
 * Cutting an edge means its endpoints land in opposite partitions.
 * @returns {{ cut: number, spins: number[], partition: [number[], number[]] }}
 */
export function maxCut(edges, n, opts = {}) {
  const J = Array.from({ length: n }, () => new Float64Array(n));
  for (const [a, b, w = 1] of edges) {
    J[a][b] -= w; // J = -W : minimizing E maximizes the cut
    J[b][a] -= w;
  }
  const { spins } = solveIsing(J, opts);
  let cut = 0;
  for (const [a, b, w = 1] of edges) if (spins[a] !== spins[b]) cut += w;
  const A = [];
  const B = [];
  for (let i = 0; i < n; i++) (spins[i] > 0 ? A : B).push(i);
  return { cut, spins: [...spins], partition: [A, B] };
}
