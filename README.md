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

## The stack

```
  arcade        room code · proximity proof   ← demo / pair / prove co-location
  ────────────────────────────────────────
  channel       one interface, many media     ← BLE · sound · light · QR · LAN
  chunk         sequenced, CRC'd, reassembled  ← survive low-bandwidth + loss
  frame         bytes ⇄ base64url ⇄ JSON · CRC ← the universal byte layer
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

13 cases: CRC vector, out-of-order + corrupted reassembly, loopback transfer,
room-code normalization, and proximity verify / noise-tolerance / anti-spoof.

## Roadmap

Real channel backends behind the `channel` contract — Web Bluetooth, an
ultrasonic `AudioContext` modem, a camera/light observer for the proximity
proof — each a drop-in `{ kind, send, subscribe }`. The pure cores here
(framing, chunking, proof) are the parts worth getting provably right first.
