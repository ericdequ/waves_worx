# transport/sonic — Ultrasonic frame + cohort HMAC schedule

Frame encoder/decoder for BEV's inaudible (18-20 kHz) data-over-sound
transport. Wire-compatible with [ggwave](https://github.com/ggerganov/ggwave)
normal-mode payload (~16 bytes after FEC).

Native side: build on ggwave (Apache 2.0). Plugin is **not yet built** — see
`docs/refactor/ULTRASONIC_VIBE.md` Phase 0 feasibility gate before committing
to native engineering.

## Lift-and-shift dependencies

Only `transport/` (sibling, for `VectorID` + fallback registration) and
`transport/internal/codec/`. Pure stdlib otherwise (`crypto/hmac` +
`crypto/sha256`).

## Frame layout (16 bytes)

```
[0]      version (1)
[1]      kind: 1=cohort chirp, 2=identity beacon, 3=handshake
[2..3]   cohort time bucket (uint16 LE)
[4..11]  HMAC-derived nonce (8 bytes)
[12..14] peer tag (3 bytes — short ephemeral)
[15]     XOR checksum of [0..14]
```

## Cohort HMAC schedule

```
nonce(t) = HMAC-SHA256(lobby_secret, "cohort\0" || uint64BE(t))[:8]
```

All synchronized emitters in the same lobby produce identical nonces for
the same second. An impostor without `lobby_secret` can't predict the next
frame. Cloud (RTDB) distributes `lobby_secret` at join.

```go
import "github.com/ericdequ/BEV/GO/mobile/bevcore/transport/sonic"

// Sender: emit a frame for this second
nonce := sonic.CohortNonce(lobbySecret, time.Now().Unix())
frame, _ := sonic.EncodeFrame(sonic.Payload{
    Kind: sonic.KindCohort,
    TimeBucket: uint16(time.Now().Unix() % 65536),
    Nonce: nonce,
    PeerTag: myShortTag,
})

// Receiver: verify someone else's frame
payload, _ := sonic.DecodeFrame(receivedBytes)
ok := sonic.VerifyCohortNonce(lobbySecret, time.Now().Unix(), 2, payload.Nonce)
```

## Fallback chain

| Platform | Chain                              | Why                                      |
|----------|------------------------------------|------------------------------------------|
| any      | `[sonic, ble, wifi_p2p]`           | Default: BLE next, then bulk transport   |
| web      | `[ble]`                            | No native sonic on web; Web Audio bg restrictions on iOS Safari + Chrome autoplay rules make it unreliable |
| ios      | `[sonic, uwb, ble]`                | UWB second on capable iPhones — same "same-room" guarantee with cleaner UX |

## Acoustic character policy

Any sound BEV emits, including game/session sonic proofs and future audible
fallback cues, must be gentle, peaceful, and pleasant. The Go capability layer
exposes this as `soundProfile: "gentle_pleasant"` and `envelope: "soft_fade"`
with low volume caps, short burst windows, quiet rest periods, and
`userIntentOnly: true`. Native plugins should treat those fields as policy, not
styling hints: no bootstrap/background surprise sounds, no sharp attacks, no
alarm-like tones, and no loud room-filling output.

## Apply ops

- `transport.encode_sonic` / `transport.decode_sonic`
- `transport.cohort_schedule` — derive nonce for `(lobby_secret, t)`
- `transport.cohort_verify` — check observed nonce with skew tolerance

## What's NOT here

- No audio emit/capture. Native plugin owns the speaker + mic.
- No FEC. ggwave handles error correction; Go only sees post-FEC frames.
- No iOS Audio background entitlement decisions. Per ULTRASONIC_VIBE.md,
  default to lobby-session-gated foreground listening to avoid App Store
  rejection.
