// Package uwb interprets UWB ranging observations (no frame encoding — UWB
// radios broadcast timestamps, not user payloads). Native frameworks (iOS
// NearbyInteraction, Android UWB API 31+) hand us distance + angles; this
// package classifies them into MeetupVerdicts + feeds the aggregator.
//
// Cross-OS reality: iPhone-iPhone via NearbyInteraction works since iOS 14.
// Android UWB exists from API 31+ but is fragmented. iOS↔Android UWB is
// possible via the FiRa Consortium spec but still rocky as of 2026.
//
// Fallback chain:
//
//	ios:     [uwb, sonic, ble]       — UWB works on iPhone 11+
//	android: [uwb, sonic, ble]       — only API 31+; otherwise sonic
//	any:     [sonic, ble]            — no UWB → fall back to sonic for same-room
package uwb

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ericdequ/waves_worx/go/transport"
)

// latest is a per-peer cache of the most recent UWBObservation. The cross-
// vector aggregator stores only the fields used for trust scoring (distance,
// confidence); azimuth/elevation are UWB-specific and live here so the
// compass page can read direction directly from UWB instead of GPS.
var (
	latestMu sync.RWMutex
	latest   = map[string]Observation{}
)

func storeLatest(o Observation) {
	if o.PeerID == "" {
		return
	}
	latestMu.Lock()
	defer latestMu.Unlock()
	if prev, ok := latest[o.PeerID]; ok && prev.ObservedAtMs > o.ObservedAtMs {
		return // ignore out-of-order arrivals
	}
	latest[o.PeerID] = o
}

// Latest returns the most recent UWB observation for peerID, or false if
// none. Used by the compass page to override GPS-derived bearing with UWB
// azimuth when fresh.
func Latest(peerID string) (Observation, bool) {
	latestMu.RLock()
	defer latestMu.RUnlock()
	o, ok := latest[peerID]
	return o, ok
}

// ClearLatest drops the per-peer cache. Hookable for tests + logout.
func ClearLatest() {
	latestMu.Lock()
	latest = map[string]Observation{}
	latestMu.Unlock()
}

func init() {
	transport.RegisterFallback(transport.VectorUWB, "ios", transport.FallbackChain{
		transport.VectorUWB,
		transport.VectorSonic,
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorUWB, "android", transport.FallbackChain{
		transport.VectorUWB,
		transport.VectorSonic,
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorUWB, "", transport.FallbackChain{
		transport.VectorSonic,
		transport.VectorBLE,
	})
}

// Observation is one ranging sample from a UWB-capable peer.
type Observation struct {
	PeerID       string  `json:"peerId"`
	DistanceM    float64 `json:"distanceM"`
	AzimuthDeg   float64 `json:"azimuthDeg"`
	ElevationDeg float64 `json:"elevationDeg"`
	Confidence   float64 `json:"confidence"` // 0..1
	ObservedAtMs int64   `json:"observedAtMs"`
}

// MeetupVerdict is the trust-tier outcome of a UWB observation.
type MeetupVerdict string

const (
	MeetupHandshake MeetupVerdict = "handshake" // ≤1m
	MeetupNearby    MeetupVerdict = "nearby"    // ≤3m
	MeetupSameRoom  MeetupVerdict = "sameRoom"  // ≤10m
	MeetupDistant   MeetupVerdict = "distant"   // >10m
)

// Classify returns the verdict for an observation. Low confidence (<0.6)
// pulls the verdict one tier down — conservative bias.
func Classify(o Observation) MeetupVerdict {
	v := MeetupDistant
	switch {
	case o.DistanceM > 0 && o.DistanceM <= 1:
		v = MeetupHandshake
	case o.DistanceM > 0 && o.DistanceM <= 3:
		v = MeetupNearby
	case o.DistanceM > 0 && o.DistanceM <= 10:
		v = MeetupSameRoom
	}
	if o.Confidence > 0 && o.Confidence < 0.6 {
		v = degrade(v)
	}
	return v
}

func degrade(v MeetupVerdict) MeetupVerdict {
	switch v {
	case MeetupHandshake:
		return MeetupNearby
	case MeetupNearby:
		return MeetupSameRoom
	default:
		return MeetupDistant
	}
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type recordInput struct {
	Observation Observation `json:"observation"`
}

type recordResult struct {
	Verdict MeetupVerdict `json:"verdict"`
	Trust   float64       `json:"trust"`
}

// ApplyRecord ingests a UWB observation: classifies it, feeds the cross-
// vector aggregator, AND stashes the full observation (with azimuth) in the
// per-peer Latest cache so the compass page can override GPS-derived bearing.
func ApplyRecord(payload string) (recordResult, error) {
	var in recordInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return recordResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	if in.Observation.PeerID == "" {
		return recordResult{}, fmt.Errorf("peerId required")
	}
	verdict := Classify(in.Observation)
	storeLatest(in.Observation)
	agg := transport.GlobalAggregator()
	agg.Record(transport.PeerObservation{
		Vector:       transport.VectorUWB,
		PeerID:       in.Observation.PeerID,
		ObservedAtMs: in.Observation.ObservedAtMs,
		DistanceM:    in.Observation.DistanceM,
		SignalQual:   in.Observation.Confidence,
	})
	return recordResult{Verdict: verdict, Trust: agg.TrustOf(in.Observation.PeerID)}, nil
}

type latestInput struct {
	PeerID string `json:"peerId"`
}

type latestResult struct {
	Observation Observation   `json:"observation"`
	Verdict     MeetupVerdict `json:"verdict"`
	Present     bool          `json:"present"`
}

// ApplyLatest returns the most recent observation for a peer, with its
// MeetupVerdict re-classified at read time. Present=false means no UWB
// observation has ever been recorded for this peer (compass should fall
// back to GPS-derived bearing).
func ApplyLatest(payload string) (latestResult, error) {
	var in latestInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return latestResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	if in.PeerID == "" {
		return latestResult{}, fmt.Errorf("peerId required")
	}
	o, ok := Latest(in.PeerID)
	if !ok {
		return latestResult{Present: false}, nil
	}
	return latestResult{
		Observation: o,
		Verdict:     Classify(o),
		Present:     true,
	}, nil
}
