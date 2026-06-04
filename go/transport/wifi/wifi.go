// Package wifi handles BEV's Wi-Fi P2P transport envelope. Cross-platform:
//
//   - Android: Wi-Fi Aware (NAN), API 26+, full service discovery.
//   - iOS:     Multipeer Connectivity (MPC), proprietary, no NAN interop.
//
// The native plugins run the right framework per platform. This Go file
// defines a JSON envelope both can carry so the Go aggregator sees a uniform
// shape regardless of which radio observed the peer.
//
// **Cross-OS gap**: Android NAN and iOS MPC do not interoperate. An Android
// phone cannot discover an iPhone over Wi-Fi P2P. Mixed-platform lobbies
// fall back to BLE + RTDB for sync.
//
// Bandwidth is plenty (Mbps) so the envelope is verbose self-describing JSON.
//
// Fallback chain:
//
//	any:     [wifi_p2p, ble]    — fall back to BLE for envelope, RTDB elsewhere
//	web:     [ble]              — no Wi-Fi P2P on web
package wifi

import (
	"encoding/json"
	"fmt"

	"github.com/ericdequ/waves_worx/go/transport"
)

func init() {
	transport.RegisterFallback(transport.VectorWiFiP2P, "", transport.FallbackChain{
		transport.VectorWiFiP2P,
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorWiFiP2P, "web", transport.FallbackChain{
		transport.VectorBLE,
	})
}

// Envelope is the cross-platform service-info / advertisement payload.
type Envelope struct {
	Version   int            `json:"v"`
	Kind      string         `json:"k"` // "identity" | "gossip" | "photo_offer" | ...
	PeerID    string         `json:"pid"`
	Geohash9  string         `json:"g9,omitempty"`
	Signature string         `json:"sig,omitempty"` // base64 Ed25519 of pid||g9||t
	IssuedAt  int64          `json:"t"`             // unix ms
	Extra     map[string]any `json:"x,omitempty"`
}

// Encode serializes the envelope as compact JSON. Result is what the native
// plugin advertises in the service-info field.
func Encode(e Envelope) ([]byte, error) {
	if e.Version == 0 {
		e.Version = 1
	}
	if e.PeerID == "" {
		return nil, fmt.Errorf("peerId required")
	}
	if e.Kind == "" {
		return nil, fmt.Errorf("kind required")
	}
	return json.Marshal(e)
}

// Decode parses a service-info payload back into the envelope.
func Decode(buf []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(buf, &e); err != nil {
		return Envelope{}, fmt.Errorf("invalid wifi p2p envelope: %w", err)
	}
	if e.PeerID == "" || e.Kind == "" {
		return Envelope{}, fmt.Errorf("peerId + kind required")
	}
	return e, nil
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type encodeInput struct {
	Envelope Envelope `json:"envelope"`
}

type encodeResult struct {
	Payload []byte `json:"payload"`
}

func ApplyEncode(payload string) (encodeResult, error) {
	var in encodeInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return encodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	out, err := Encode(in.Envelope)
	if err != nil {
		return encodeResult{}, err
	}
	return encodeResult{Payload: out}, nil
}

type decodeResult struct {
	Envelope Envelope `json:"envelope"`
}

func ApplyDecode(payload string) (decodeResult, error) {
	var in struct {
		Payload []byte `json:"payload"`
	}
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	e, err := Decode(in.Payload)
	if err != nil {
		return decodeResult{}, err
	}
	return decodeResult{Envelope: e}, nil
}
