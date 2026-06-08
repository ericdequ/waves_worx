# AD2/wavecompute — wave dynamics that solve real math

Using **wave/oscillator capabilities to run the algorithms quantum is famous
for** — classically, on the problem classes where physics gives a real speedup.
Standalone, pure, zero-dep, node-tested.

```bash
node --test ising.test.mjs       # the machine matches brute-force MAX-CUT on every graph
node demo.mjs                    # watch the oscillator network settle into the answer
node --test wavefourier.test.mjs # the wave DFT — incl. proof it equals the quantum QFT
node fourier-demo.mjs            # a wave's spectrum + the convolution speedup
```

## What's here: an oscillator Ising machine (Simulated Bifurcation)

Each variable is a nonlinear **oscillator** (position + momentum). A pump energy
is ramped up; the network **bifurcates** and every oscillator falls to +1 or −1 —
and the configuration it lands in is the **minimum-energy state** of the Ising
problem. That solves **MAX-CUT / QUBO** — the same NP-hard combinatorial class
**quantum annealers** (D-Wave) target. Simulated Bifurcation is the classical
algorithm that competes with them, and it runs great on a GPU.

```js
import { maxCut } from './ising.js';
maxCut([[0,1],[1,2],[2,3],[3,0]], 4);   // → { cut: 4, partition: [[0,2],[1,3]] }
```

The tests prove the honest claim: the machine's cut **equals the exhaustive
brute-force optimum** on every test graph (square, frustrated triangle, pentagon,
K4, a random 8-node graph) — wave relaxation reaching the exact answer without
enumerating states.

## The honest framing

This is the real "quantum-like speedup": the optimization is done by the
**physics of relaxation**, not by trying configurations. It's classical — n
variables cost n oscillators, not 2ⁿ — and it shines on *specific* math
(optimization, transforms, MVM), not universally. That restriction is exactly
why it's quantum-*like*, not quantum.

## Wave Fourier transform (`wavefourier.js`) — built

A propagating wave *is* a Fourier transform: its spectrum is the DFT, computed by
projecting the wave onto every frequency at once (literally the AD2 Goertzel from
`../src/dsp.js`, run across the whole band). The headline capability is the
**convolution theorem** — convolution in time becomes a pointwise *multiply* in
frequency, turning an O(N²) operation into O(N). And the tests **prove** this is
the same operation as the quantum **QFT** (`../quantum/qft()`): the classical wave
DFT inverts the quantum Fourier transform exactly.

```js
import { dft, convolve } from './wavefourier.js';
dft(samples);            // spectrum = the wave's Fourier transform
convolve(signal, kernel); // filter, via multiply-in-frequency (the speedup)
```

## Interference MVM (`interferenceMvm.js`) — built

A matrix-vector multiply done by wave interference: the input vector is encoded
as waves and each output is the coherent sum (constructive/destructive) weighted
by a matrix row — all outputs at once, in one pass (the optical-neural-net core).
A negative value is a π-phase wave, so subtraction is cancellation. The tests
prove the DFT is just **one special configuration** of this mesh
(`mvm(dftMatrix, x) === dft(x)`).

```js
import { mvm, dftMatrix } from './interferenceMvm.js';
mvm([[2,0,1],[1,3,0],[0,1,2]], [1,2,3]);   // a matrix-vector product, by interference
```

## Reservoir computing (`reservoir.js`) — built

A fixed, random, recurrent **nonlinear medium** (an Echo State Network) that you
do *not* train — only a cheap linear readout learns. The medium's dynamics make
temporal patterns linearly separable. The tests run the classic **delayed-XOR**
task (needs memory *and* nonlinearity): the reservoir scores >90% while a linear
readout on the raw inputs is stuck near chance — the gap is the whole point.

```js
import { createReservoir, runStream, ridgeFit } from './reservoir.js';
// fixed reservoir provides features; ridge readout is the only trained part.
```

## All six wave-native kernels are built

| kernel | quantum cousin | here |
| --- | --- | --- |
| oscillator Ising machine | quantum annealing | `ising.js` |
| Grover amplitude amplification | Grover | `../quantum` |
| quantum gates as interference | gate model | `../quantum` |
| wave Fourier transform | QFT | `wavefourier.js` (proven == QFT) |
| interference MVM | quantum linear algebra | `interferenceMvm.js` |
| reservoir computing | quantum reservoirs | `reservoir.js` |

## References

- **Ising machine** — H. Goto, K. Tatsumura, A. R. Dixon, "Combinatorial
  optimization by simulating adiabatic bifurcations in nonlinear Hamiltonian
  systems," *Science Advances* 5, eaav2372 (2019); ballistic/discrete SB: Goto
  et al., *Science Advances* 7, eabe7953 (2021).
- **Wave Fourier transform** — J. W. Goodman, *Introduction to Fourier Optics*
  (1968); G. Goertzel, *Amer. Math. Monthly* 65 (1958).
- **Interference MVM** — Y. Shen et al., "Deep learning with coherent
  nanophotonic circuits," *Nature Photonics* 11, 441–446 (2017).
- **Reservoir computing** — H. Jaeger, "The echo state approach…," GMD Report 148
  (2001); M. Lukoševičius & H. Jaeger, *Computer Science Review* (2009).

## See also

- [`../quantum/`](../quantum) — Grover + the QFT (the wave FT's quantum twin)
- [`../README.md`](../README.md) — the wave-compute suite gateway
- [`../../go/mlvswave`](../../go/mlvswave) — measured ML-vs-wave-compute comparison
