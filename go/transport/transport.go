// Package transport is the shared trust + observation layer for BEV's
// six broadcast/peer data transports. Each transport is its own subpackage
// (transport/ble, transport/sonic, …) so individual transports can be
// lifted into other projects with minimal churn — they depend only on this
// shared types package plus the stdlib.
//
// Architecture:
//
//	core.go (bevcore root)
//	  └─ dispatches Apply ops to subpackages
//	         ├─ transport         ← this package: VectorID, Aggregator,
//	         │                      Capability, FallbackOf, record/peer_trust
//	         ├─ transport/ble
//	         ├─ transport/sonic
//	         ├─ transport/nfc
//	         ├─ transport/uwb
//	         ├─ transport/wifi
//	         └─ transport/vlc
//
// Per-vector packages each:
//   - Define their frame layout + encoder/decoder
//   - Register a Fallback chain at init()
//   - Export ApplyEncode / ApplyDecode for the bevcore dispatch
//   - Carry their own tests
//
// The Go middle-end does not own any radios. Native Capacitor plugins
// (vendor/capacitor-go-core/*) push observations through the Aggregator
// via the `transport.record_observation` Apply op.
package transport

// ─── Vector identity + capability ───────────────────────────────────────────

// VectorID enumerates the broadcast/peer transport vectors the aggregator
// recognizes. Add a new vector by:
//  1. Declaring its VectorID const here
//  2. Adding its base trust weight to BaseTrustWeight()
//  3. Adding its max-frame size to MaxBytesPerFrame()
//  4. Creating a transport/<name>/ subpackage with Apply ops + init() fallback
type VectorID string

const (
	VectorBLE     VectorID = "ble"
	VectorSonic   VectorID = "sonic"
	VectorWiFiP2P VectorID = "wifi_p2p"
	VectorUWB     VectorID = "uwb"
	VectorVLC     VectorID = "vlc"
	VectorNFC     VectorID = "nfc"
)

// AllVectors lists every recognized vector. Order is the canonical priority
// order for diagnostics — trust math is unordered.
var AllVectors = []VectorID{
	VectorBLE,
	VectorSonic,
	VectorWiFiP2P,
	VectorUWB,
	VectorVLC,
	VectorNFC,
}

// IsKnown reports whether v is a recognized vector ID. Used to reject
// misbehaving platform plugins reporting fictional vectors.
func (v VectorID) IsKnown() bool {
	switch v {
	case VectorBLE, VectorSonic, VectorWiFiP2P, VectorUWB, VectorVLC, VectorNFC:
		return true
	}
	return false
}

// BaseTrustWeight returns the inherent trust contribution of one observation
// on this vector. Higher = stronger physical-proof. Pinned by test against
// docs/refactor/TRANSPORT_VECTORS.md — changes must update the doc too.
func (v VectorID) BaseTrustWeight() float64 {
	switch v {
	case VectorNFC:
		return 1.00
	case VectorUWB:
		return 0.95
	case VectorVLC:
		return 0.85
	case VectorSonic:
		return 0.80
	case VectorWiFiP2P:
		return 0.55
	case VectorBLE:
		return 0.40
	}
	return 0
}

// MaxBytesPerFrame is the practical per-frame user-data budget on this vector,
// after error correction. Wire bytes are higher (vector-specific framing).
func (v VectorID) MaxBytesPerFrame() int {
	switch v {
	case VectorBLE:
		return 21
	case VectorSonic:
		return 16
	case VectorWiFiP2P:
		return 255
	case VectorUWB:
		return 32
	case VectorVLC:
		return 64
	case VectorNFC:
		return 256
	}
	return 0
}

// Capability is reported by the native platform plugin via Apply op
// `transport.report_capability` — the platform tells us once at boot whether
// each vector actually works on this device. Go-side never knows on its own.
type Capability struct {
	Vector    VectorID `json:"vector"`
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`   // human-readable, e.g. "speaker rolloff <18kHz"
	Platform  string   `json:"platform,omitempty"` // "ios" | "android" | "web" | ""
}
