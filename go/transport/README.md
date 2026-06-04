# bevcore/transport — Six-vector aggregator + per-vector frame encoders

BEV's Go middle-end is an **aggregator across six broadcast/peer transports**.
Each transport lives in its own subpackage so it can be lifted and reused
independently. The shared `transport` package owns the cross-vector trust
math + the fallback registry.

## Subpackages

| Path                          | Purpose                                       |
|-------------------------------|-----------------------------------------------|
| `transport/`                  | `VectorID`, `Aggregator`, `Capability`, fallback registry |
| `transport/ble/`              | 21-byte BLE manufacturer-data frame           |
| `transport/sonic/`            | 16-byte ultrasonic frame + cohort HMAC schedule |
| `transport/nfc/`              | 108-byte NDEF handshake record (Android-only radio) |
| `transport/uwb/`              | UWB observation classifier (no frame; radios speak ranges) |
| `transport/wifi/`             | JSON service-info envelope (NAN + MPC)        |
| `transport/vlc/`              | Chunked screen-to-camera payload (fun mode)   |
| `transport/internal/codec/`   | Shared wire helpers (XOR, geohash alphabet)   |

## Reuse + extend

Each per-vector subpackage depends only on:

- `transport/` (sibling, for `VectorID` + fallback registration)
- `transport/internal/codec/` if it needs the shared helpers
- Go stdlib

You can lift any subpackage into another project by copying its directory
plus `transport/` + `internal/codec/` and adjusting import paths. None of
them reach into bevcore's other internals.

To add a new vector:

1. Add a `VectorID` const + `BaseTrustWeight` + `MaxBytesPerFrame` entry in `transport/transport.go`
2. Create `transport/<name>/` with `<name>.go` + `<name>_test.go` + `README.md`
3. Call `transport.RegisterFallback(...)` in your package's `init()`
4. Export `ApplyEncode`/`ApplyDecode` (or whatever Apply ops you need)
5. Wire your Apply ops into bevcore's `core.go` switch

## Cross-vector trust math

A peer observed on N distinct vectors within 60 seconds gets:

```
score = (sum of per-vector best-evidence quality) * (1 + 0.5*(N-1)) / N
score = min(score, 1.0)
```

So:
- 1 vector → average quality
- 2 vectors → 1.5× boost on the average
- 3 vectors → 2.0× boost, capped at 1.0 — effectively "verified physical co-presence"

Pinned by `TestAggregatorCrossVectorBoost` and `TestAggregatorBoostCapsAtOne`.

## Fallback contracts

Each transport registers its fallback chain at `init()` so anyone calling
`transport.FallbackOf(VectorX, platform)` gets back the right ordered list
of vectors to try. Platform-aware chains let NFC route around iOS (where
peer mode is dead since iOS 13) and let Sonic route around web (where
the native sonic plugin doesn't exist).

`SetPlatform("ios")` once at boot via `transport.set_platform` so all
later `FallbackOf` calls use the right chain.

## Apply ops surfaced through bevcore dispatch

Cross-vector (in `transport/`):
- `transport.record_observation`
- `transport.peer_trust`
- `transport.all_peers`
- `transport.set_platform`
- `transport.fallback_of`

Per-vector (each subpackage):
- `transport.encode_<vector>` / `transport.decode_<vector>`
- `transport.cohort_schedule` / `transport.cohort_verify` (sonic only)
- `transport.record_uwb` (uwb only — radios speak ranges, not frames)
