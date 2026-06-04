package transport

import (
	"encoding/json"
	"errors"
	"fmt"
)

type RecordObservationInput struct {
	Vector       string  `json:"vector"`
	PeerID       string  `json:"peerId"`
	Payload      []byte  `json:"payload,omitempty"`
	ObservedAtMs int64   `json:"observedAtMs,omitempty"`
	DistanceM    float64 `json:"distanceM,omitempty"`
	RSSI         int     `json:"rssi,omitempty"`
	SignalQual   float64 `json:"signalQual,omitempty"`
}

type RecordObservationResult struct {
	Recorded bool    `json:"recorded"`
	Trust    float64 `json:"trust"`
}

// ApplyRecordObservation handles transport.record_observation.
func ApplyRecordObservation(payload string) (RecordObservationResult, error) {
	var in RecordObservationInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return RecordObservationResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	v := VectorID(in.Vector)
	if !v.IsKnown() {
		return RecordObservationResult{}, fmt.Errorf("unknown vector %q", in.Vector)
	}
	if in.PeerID == "" {
		return RecordObservationResult{}, errors.New("peerId required")
	}
	agg := GlobalAggregator()
	agg.Record(PeerObservation{
		Vector:       v,
		PeerID:       in.PeerID,
		Payload:      in.Payload,
		ObservedAtMs: in.ObservedAtMs,
		DistanceM:    in.DistanceM,
		RSSI:         in.RSSI,
		SignalQual:   in.SignalQual,
	})
	return RecordObservationResult{Recorded: true, Trust: agg.TrustOf(in.PeerID)}, nil
}

type PeerTrustInput struct {
	PeerID string `json:"peerId"`
}

// ApplyPeerTrust handles transport.peer_trust.
func ApplyPeerTrust(payload string) (TrustView, error) {
	var in PeerTrustInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return TrustView{}, fmt.Errorf("invalid payload: %w", err)
	}
	if in.PeerID == "" {
		return TrustView{}, errors.New("peerId required")
	}
	return GlobalAggregator().View(in.PeerID), nil
}

type AllPeersResult struct {
	Peers []TrustView `json:"peers"`
}

// ApplyAllPeers handles transport.all_peers.
func ApplyAllPeers(_ string) (AllPeersResult, error) {
	return AllPeersResult{Peers: GlobalAggregator().AllPeers()}, nil
}

type SetPlatformInput struct {
	Platform string `json:"platform"`
}

type SetPlatformResult struct {
	Set string `json:"set"`
}

// ApplySetPlatform handles transport.set_platform.
func ApplySetPlatform(payload string) (SetPlatformResult, error) {
	var in SetPlatformInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return SetPlatformResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	SetPlatform(in.Platform)
	return SetPlatformResult{Set: in.Platform}, nil
}

type FallbackOfInput struct {
	Vector   string `json:"vector"`
	Platform string `json:"platform,omitempty"`
}

type FallbackOfResult struct {
	Chain FallbackChain `json:"chain"`
}

// ApplyFallbackOf handles transport.fallback_of.
func ApplyFallbackOf(payload string) (FallbackOfResult, error) {
	var in FallbackOfInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return FallbackOfResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	platform := in.Platform
	if platform == "" {
		platform = Platform()
	}
	return FallbackOfResult{Chain: FallbackOf(VectorID(in.Vector), platform)}, nil
}
