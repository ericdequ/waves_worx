package transport

import (
	"sort"
	"sync"
	"time"
)

// PeerObservation is one observation of a remote peer on one vector, fed to
// the Aggregator by either a platform plugin (live radio) or a Go encoder
// (testing). All fields are FFI-friendly primitives so this struct
// roundtrips cleanly through bevcore's Apply JSON pipe.
type PeerObservation struct {
	Vector       VectorID `json:"vector"`
	PeerID       string   `json:"peerId"`
	Payload      []byte   `json:"payload,omitempty"`
	ObservedAtMs int64    `json:"observedAtMs"`
	DistanceM    float64  `json:"distanceM,omitempty"`  // UWB; 0 = unknown
	RSSI         int      `json:"rssi,omitempty"`       // BLE/Wi-Fi; 0 = unknown
	SignalQual   float64  `json:"signalQual,omitempty"` // 0..1 normalized; 0 = unknown
}

// TrustView is the Aggregator's per-peer summary across all vectors.
type TrustView struct {
	PeerID       string               `json:"peerId"`
	Score        float64              `json:"score"`
	ByVector     map[VectorID]float64 `json:"byVector"`
	Observations int                  `json:"observations"`
	LastSeenMs   int64                `json:"lastSeenMs"`
}

// Aggregator collects observations across all vectors and computes per-peer
// trust. Safe for concurrent use.
//
// Cross-vector boost: a peer observed on N distinct vectors within
// CrossVectorWindowMs gets boost (1 + 0.5*(N-1)) on its normalized
// base-weighted average, capped at 1.0. Three distinct vectors means strong
// physical co-presence evidence.
type Aggregator struct {
	mu                  sync.Mutex
	obs                 []PeerObservation
	ObsTTLMs            int64
	MaxObs              int
	CrossVectorWindowMs int64
	NowFunc             func() int64 // hookable for tests
}

// NewAggregator constructs an aggregator with default tunables:
//   - ObsTTLMs: 10 minutes
//   - MaxObs: 1024
//   - CrossVectorWindowMs: 60 seconds
func NewAggregator() *Aggregator {
	return &Aggregator{
		ObsTTLMs:            10 * 60 * 1000,
		MaxObs:              1024,
		CrossVectorWindowMs: 60 * 1000,
		NowFunc:             func() int64 { return time.Now().UnixMilli() },
	}
}

// Record adds an observation. Unknown vectors and empty peer IDs are
// silently dropped; bevcore is a library, not a daemon.
func (a *Aggregator) Record(o PeerObservation) {
	if !o.Vector.IsKnown() || o.PeerID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if o.ObservedAtMs == 0 {
		o.ObservedAtMs = a.NowFunc()
	}
	a.obs = append(a.obs, o)
	a.prune()
}

func (a *Aggregator) prune() {
	cutoff := a.NowFunc() - a.ObsTTLMs
	start := 0
	for start < len(a.obs) && a.obs[start].ObservedAtMs < cutoff {
		start++
	}
	if start > 0 {
		a.obs = a.obs[start:]
	}
	if len(a.obs) > a.MaxObs {
		a.obs = a.obs[len(a.obs)-a.MaxObs:]
	}
}

// TrustOf returns the current trust score for a peer in [0, 1].
func (a *Aggregator) TrustOf(peerID string) float64 {
	if peerID == "" {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.computeTrust(peerID)
}

// View returns the full TrustView for a peer.
func (a *Aggregator) View(peerID string) TrustView {
	if peerID == "" {
		return TrustView{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	byVector := map[VectorID]float64{}
	var lastSeen int64
	obsCount := 0
	for _, o := range a.obs {
		if o.PeerID != peerID {
			continue
		}
		obsCount++
		if o.ObservedAtMs > lastSeen {
			lastSeen = o.ObservedAtMs
		}
		q := a.observationQuality(o)
		if q > byVector[o.Vector] {
			byVector[o.Vector] = q
		}
	}
	return TrustView{
		PeerID:       peerID,
		Score:        a.computeTrust(peerID),
		ByVector:     byVector,
		Observations: obsCount,
		LastSeenMs:   lastSeen,
	}
}

// observationQuality returns the per-observation contribution to that
// vector's best-evidence quality, with per-vector modifiers.
func (a *Aggregator) observationQuality(o PeerObservation) float64 {
	base := o.Vector.BaseTrustWeight()
	if base == 0 {
		return 0
	}
	switch o.Vector {
	case VectorUWB:
		if o.DistanceM <= 0 {
			return base * 0.5
		}
		switch {
		case o.DistanceM <= 1:
			return base * 1.0
		case o.DistanceM <= 5:
			return base * 0.9
		case o.DistanceM <= 10:
			return base * 0.7
		default:
			return base * 0.4
		}
	case VectorBLE, VectorWiFiP2P:
		if o.RSSI == 0 {
			return base * 0.8
		}
		switch {
		case o.RSSI >= -50:
			return base
		case o.RSSI >= -70:
			return base * 0.85
		case o.RSSI >= -85:
			return base * 0.6
		default:
			return base * 0.4
		}
	case VectorSonic:
		if o.SignalQual <= 0 {
			return base * 0.7
		}
		if o.SignalQual >= 0.9 {
			return base
		}
		return base * o.SignalQual
	}
	return base
}

// computeTrust does the cross-vector consensus math. Must be called under a.mu.
func (a *Aggregator) computeTrust(peerID string) float64 {
	now := a.NowFunc()
	windowStart := now - a.CrossVectorWindowMs

	byVector := map[VectorID]float64{}
	for _, o := range a.obs {
		if o.PeerID != peerID || o.ObservedAtMs < windowStart {
			continue
		}
		q := a.observationQuality(o)
		if q > byVector[o.Vector] {
			byVector[o.Vector] = q
		}
	}
	if len(byVector) == 0 {
		return 0
	}
	sum := 0.0
	for _, q := range byVector {
		sum += q
	}
	boost := 1 + 0.5*float64(len(byVector)-1)
	score := sum * boost / float64(len(byVector))
	if score > 1 {
		score = 1
	}
	return score
}

// AllPeers returns every peer with at least one observation, sorted by trust
// descending.
func (a *Aggregator) AllPeers() []TrustView {
	a.mu.Lock()
	peers := map[string]struct{}{}
	for _, o := range a.obs {
		peers[o.PeerID] = struct{}{}
	}
	views := make([]TrustView, 0, len(peers))
	for pid := range peers {
		v := TrustView{PeerID: pid, Score: a.computeTrust(pid), ByVector: map[VectorID]float64{}}
		for _, o := range a.obs {
			if o.PeerID != pid {
				continue
			}
			v.Observations++
			if o.ObservedAtMs > v.LastSeenMs {
				v.LastSeenMs = o.ObservedAtMs
			}
			q := a.observationQuality(o)
			if q > v.ByVector[o.Vector] {
				v.ByVector[o.Vector] = q
			}
		}
		views = append(views, v)
	}
	a.mu.Unlock()
	sort.Slice(views, func(i, j int) bool { return views[i].Score > views[j].Score })
	return views
}

// Clear empties the aggregator. Used on logout / "forget peers."
func (a *Aggregator) Clear() {
	a.mu.Lock()
	a.obs = nil
	a.mu.Unlock()
}
