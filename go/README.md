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
ad2ml                stdlib AD2/Pidar feature extraction for Go ML/EPU adapters
mlvswave             CPU-safe ML vs wave-compute accuracy/latency experiment
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

AD2/Pidar feature extraction:

```go
import "github.com/ericdequ/waves_worx/go/ad2ml"

features := ad2ml.SenseFeatures(samples, 48000, ad2ml.DefaultConfig())
tensorish := features.FeatureVector32()
```

`ad2ml` is intentionally stdlib-only. It gives Gonum, GoMLX, Python tensor jobs,
and future EPU services a stable feature shape without requiring those heavier
libraries in the core module.

ML vs wave-compute comparison:

```bash
env GOCACHE=/tmp/go-build-cache go test -v ./mlvswave
env GOCACHE=/tmp/go-build-cache go test -bench . ./mlvswave
```

`mlvswave` keeps the comparison honest on an RX 480 era machine: the classical
path is Goertzel argmax, while the ML path learns from the same band-energy
features. GPU acceleration can replace the classifier later, but the baseline
accuracy/latency harness remains the same.

The `Apply*` functions take/return JSON strings, so the module drops behind a
single string-in/string-out bridge (gomobile, WASM `syscall/js`, or a Cloudflare
Worker) — the same op-dispatch shape the rest of the ecosystem uses.

## Relationship to BEV

BEV still carries its own copy of this code under `GO/mobile/bevcore/transport`,
now decoupled from `bevcore` (registry inversion — `bevcore` has 0 transport
deps). The intended cutover: publish this module, have BEV `require` it, and
delete the BEV copy. Until then the two are identical Go; keep changes in sync.
