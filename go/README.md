# waves_worx/go — the Go mesh core

`github.com/ericdequ/waves_worx/go` — the native/edge half of waves_worx: a
stdlib-only, zero-dependency Go module for no-cloud peer transport across BLE,
ultrasonic sound, NFC, UWB, Wi-Fi, and visible-light channels, with a
peer-trust aggregator and a transport-fallback ladder.

Lifted from BEV's `bevcore/transport` once it was decoupled from the BEV core
(it was always a clean, stdlib-only leaf). This is the polyglot sibling of the
pure-JS cores in the parent repo: the JS side (`../src`) iterates fast for web /
workers; this Go side hardens for native iOS/Android, WASM, and edge — the same
way TST ships Go + JS.

## Packages

```
transport            peer-trust Aggregator, transport state, fallback ladder,
                     Apply* ops (record observation, peer trust, set platform…)
transport/ble        BLE frame encode/decode
transport/sonic      ultrasonic channels: AEAD, capability tiers, channel
                     health/allocation, + internal/audiomodem (FSK)
transport/nfc        NFC handshake encode/decode
transport/uwb        UWB ranging record/latest
transport/wifi       Wi-Fi LAN encode/decode
transport/vlc        visible-light comms encode/decode
```

## Use

```bash
go get github.com/ericdequ/waves_worx/go
go test ./...
```

```go
import "github.com/ericdequ/waves_worx/go/transport"

agg := transport.NewAggregator()
// record peer observations across vectors, read fused trust, pick a fallback…
```

The `Apply*` functions take/return JSON strings, so the module drops behind a
single string-in/string-out bridge (gomobile, WASM `syscall/js`, or a Cloudflare
Worker) — the same op-dispatch shape the rest of the ecosystem uses.

## Relationship to BEV

BEV still carries its own copy of this code under `GO/mobile/bevcore/transport`,
now decoupled from `bevcore` (registry inversion — `bevcore` has 0 transport
deps). The intended cutover: publish this module, have BEV `require` it, and
delete the BEV copy. Until then the two are identical Go; keep changes in sync.
