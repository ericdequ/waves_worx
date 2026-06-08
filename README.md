# waves_worx

An R&D lab for **no-cloud, physical-layer data transport** between co-present
devices — plus a **games arcade** that turns physical-security concepts into
something you can hold in your hands and demo.

> **Scope & intent.** Everything here is for **authorized security testing,
> teaching, CTF-style demos, and privacy-preserving local data transfer between
> consenting, co-present devices.** Every primitive keeps data *in the room* —
> there is no cloud in the path. The proximity proof is **anti-spoofing for
> consensual pairing** (both parties opt in), not a tool for covert signalling
> or exfiltration.

Extracted and generalized from BEV's `bevnet` transport layer and its
multiplayer game-pairing handshake (which shipped with a parity test suite).
Pure JavaScript, zero dependencies, node-testable.

> **Also in this repo:** [`AD2/`](AD2) grew into a small **wave-computing
> curriculum** — a physical-layer sensing backend (the Analog Discovery 2 bench)
> and then how far classical waves reach toward quantum-class math: gate
> emulation, Grover, the QFT, an Ising machine, interference MVM, and reservoir
> computing. **Start at [`AD2/README.md`](AD2/README.md).** The Go side adds
> `go/ad2ml` (feature extraction) and `go/mlvswave` (wave-compute vs ML).

## The stack

```
  arcade        room code · proximity proof · pairing   ← pair / prove co-location
  ──────────────────────────────────────────────────
  modem         bytes ⇄ FSK tones                       ← send data over sound
  channel       one interface, many media               ← BLE · sound · light · QR · LAN
  chunk         sequenced, CRC'd, reassembled            ← survive low-bandwidth + loss
  frame         bytes ⇄ base64url ⇄ JSON · CRC          ← the universal byte layer
```

### frame — the universal byte layer

`toBytes` / `bytesToBase64Url` / `crc32` — coerce anything to bytes, carry it
through text/QR channels, integrity-check it. (CRC-32 IEEE; verified against the
canonical `0xcbf43926` test vector.)

### chunk — transport over a constrained, lossy channel

A QR code or ultrasonic burst carries only a few bytes and drops frames.
`chunk()` splits a payload into sequenced, CRC-tagged frames; a `Reassembler`
collects them **in any order**, rejects corrupted or duplicate frames, and tells
you exactly which indices are still missing.

```js
import { chunk, Reassembler } from 'waves_worx';
const r = new Reassembler();
for (const frame of chunk(payload, { id: 'job', size: 180 })) send(frame);
// receiver: r.add(frame) … r.complete … r.bytes()
```

### channel — one interface over every local medium

`CHANNEL_KINDS` records each medium's physical properties (range, bandwidth,
duplex, line-of-sight, needs-native); `pickChannel()` chooses the best
available; `createLoopbackChannel()` models "devices in radio range" for tests
and single-process demos. Implement `send` / `subscribe` to add a real BLE /
sound / light / WebRTC backend.

### modem — send bytes over sound

A frequency-shift-keying codec: each nibble becomes one of 16 tones, a preamble
syncs the receiver, a checksum byte guards integrity. It's **pure** — bytes →
`{ freqHz, durationMs }` tones and back — so it's fully testable without audio
hardware (a real speaker/mic is a thin adapter on top).

```js
import { encode, decode } from 'waves_worx';
const tones = encode('meet at 9');          // → play these through a speaker
const out = decode(observedFreqs);          // ← from a mic pitch tracker
// out → { ok, bytes, reason }   tolerant of frequency jitter; checksum-guarded
```

### pairing — establish a session, no cloud

A three-message handshake (offer → answer → confirm) two devices run over any
channel to agree on a shared session id bound to the room code. Side-effect-free
state machine: feed in received messages, get the next message to send.

```js
import { createPairing } from 'waves_worx';
const a = createPairing({ roomCode, role: 'initiator' });
const b = createPairing({ roomCode, role: 'responder' });
// a.start() → b.receive() → a.receive() → b.receive()  ⇒ a.session === b.session
```

Both sides derive the **same** session id only by completing the exchange; a
wrong room code or tampered confirm fails closed. Combine with the proximity
proof when you also need to prove physical co-location.

### arcade — prove you're actually here

The headline demo. One device **flashes** a light/screen pattern (preamble +
secret-derived data bits + checksum); the other **watches** it with a camera and
verifies. Because the pattern is derived from a shared room code **and** a
session token, only a device truly observing the screen can reproduce it — the
proof can't be relayed across the internet.

```js
import { generateRoomCode, createProximityProof, verifyProximityProof } from 'waves_worx';

const roomCode = generateRoomCode();           // "K7QF9M"
const proof = await createProximityProof({ roomCode, token });  // displayer flashes proof.frames
const result = await verifyProximityProof({ roomCode, token, observedBits });
// → { ok, reason, confidence }   ok requires confidence ≥ 0.9
```

The verifier tolerates a few misread frames (real cameras are noisy) but rejects
a wrong token outright — the anti-spoof property.

## Test

```bash
node --test test/*.test.mjs
```

22 cases: CRC vector, out-of-order + corrupted reassembly, loopback transfer,
modem round-trip / jitter-tolerance / checksum, pairing handshake + fail-closed,
room-code normalization, and proximity verify / noise-tolerance / anti-spoof.

## Roadmap

- **Real channel backends** behind the `channel` contract — Web Bluetooth, an
  ultrasonic `AudioContext` driver for the modem (`encode` → oscillator,
  mic FFT → `decode`), a camera/light observer for the proximity proof — each a
  drop-in `{ kind, send, subscribe }`.
- **AD2 bench backend — landed** (`AD2/`). A Digilent Analog Discovery 2 plays
  the modem's tones out of the AWG and captures them on the scope; pure-JS
  Goertzel recovers them (`AD2/src/dsp.js`) and `senseFeatures` extracts a
  Pidar-style feature map. The DSP is node-testable against a `createMockDevice`
  synthesizer; `AD2/python/ad2_driver.py` is the thin real-hardware adapter. See
  [`AD2/README.md`](AD2/README.md) — also the working model of "Pidar sensing".
- **BLE game transfer frames — landed** (`src/blegame.js`). A pure 21-byte frame
  codec models BLE manufacturer-data-sized arcade/game payloads, with detect /
  decode tests for corruption, out-of-order transfers, and loopback delivery.
- **Go mesh core — landed** (`go/`, module `github.com/ericdequ/waves_worx/go`).
  BEV's `bevcore/transport` was decoupled from the BEV core (registry inversion;
  `bevcore` now has 0 transport deps) and lifted here as a standalone stdlib-only
  Go module: BLE / sonic / NFC / UWB / Wi-Fi / VLC channels + a peer-trust
  aggregator + fallback ladder. 8 packages, all tests pass. See [`go/README.md`](go/README.md).
  The JS cores here (framing, chunking, modem, proof, pairing) are the contract
  the Go side matches — polyglot, like TST. Remaining cutover: publish `go/` →
  BEV `require`s it → delete BEV's copy.

## Improving this library

This library is meant to keep getting **better and more versatile through use**.
When you adopt it in a project and hit a gap — a missing variant, an awkward
API, a pattern worth generalizing — don't work around it locally:

1. Note it under **Usage learnings** below (or open an issue on this repo).
2. When the value is clear, **extend the library** (new export / variant / game /
   contract), add a test, then update the consumer. Prefer composition over
   variant sprawl, and keep it tested.

### Usage learnings

- 2026-06-07 · BEV/AD2 — added `AD2/` as the first real channel backend (Digilent
  Analog Discovery 2). Confirmed the "pure codec + thin adapter" pattern holds at
  the hardware layer: the existing `modem.encode/decode` needed zero changes — the
  backend only adds samples→tones recovery (Goertzel) and a device contract whose
  mock synthesizes what a wire loopback would capture. Doubles as the `Pidar
  sensing` model (signal → classical feature map → meaning, vectors deferred).
