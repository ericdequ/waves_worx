package transport

import "testing"

func TestVectorTrustWeightsMatchDoc(t *testing.T) {
	cases := []struct {
		v VectorID
		w float64
	}{
		{VectorNFC, 1.00},
		{VectorUWB, 0.95},
		{VectorVLC, 0.85},
		{VectorSonic, 0.80},
		{VectorWiFiP2P, 0.55},
		{VectorBLE, 0.40},
	}
	for _, c := range cases {
		if got := c.v.BaseTrustWeight(); got != c.w {
			t.Errorf("%s weight=%v, want %v (must match TRANSPORT_VECTORS.md)", c.v, got, c.w)
		}
	}
}

func TestUnknownVectorWeightZero(t *testing.T) {
	if v := VectorID("bogus").BaseTrustWeight(); v != 0 {
		t.Errorf("unknown vector weight = %v, want 0", v)
	}
}

func TestAggregatorSingleVectorBLE(t *testing.T) {
	a := NewAggregator()
	now := int64(500_000)
	a.NowFunc = func() int64 { return now }
	a.Record(PeerObservation{Vector: VectorBLE, PeerID: "p1", ObservedAtMs: now, RSSI: -60})
	score := a.TrustOf("p1")
	if score < 0.30 || score > 0.40 {
		t.Errorf("single BLE trust = %v, expected ~0.34", score)
	}
}

func TestAggregatorCrossVectorBoost(t *testing.T) {
	a := NewAggregator()
	now := int64(1_000_000)
	a.NowFunc = func() int64 { return now }
	a.Record(PeerObservation{Vector: VectorBLE, PeerID: "p1", ObservedAtMs: now, RSSI: -60})
	a.Record(PeerObservation{Vector: VectorSonic, PeerID: "p1", ObservedAtMs: now, SignalQual: 0.95})
	a.Record(PeerObservation{Vector: VectorUWB, PeerID: "p1", ObservedAtMs: now, DistanceM: 0.5, SignalQual: 0.9})
	score := a.TrustOf("p1")
	if score < 0.85 {
		t.Errorf("three-vector trust = %v, expected > 0.85", score)
	}
}

func TestAggregatorBoostCapsAtOne(t *testing.T) {
	a := NewAggregator()
	now := int64(2_000_000)
	a.NowFunc = func() int64 { return now }
	for _, v := range AllVectors {
		o := PeerObservation{Vector: v, PeerID: "p1", ObservedAtMs: now}
		switch v {
		case VectorBLE, VectorWiFiP2P:
			o.RSSI = -50
		case VectorUWB:
			o.DistanceM = 0.5
			o.SignalQual = 0.9
		case VectorSonic:
			o.SignalQual = 0.95
		}
		a.Record(o)
	}
	score := a.TrustOf("p1")
	if score > 1.0 || score < 0.9 {
		t.Errorf("all-vector trust = %v, expected 0.9-1.0 (capped)", score)
	}
}

func TestAggregatorPrunesStale(t *testing.T) {
	a := NewAggregator()
	now := int64(10_000_000)
	a.NowFunc = func() int64 { return now }
	old := now - a.ObsTTLMs - 1000
	a.Record(PeerObservation{Vector: VectorBLE, PeerID: "p1", ObservedAtMs: old, RSSI: -50})
	a.Record(PeerObservation{Vector: VectorSonic, PeerID: "p1", ObservedAtMs: now, SignalQual: 0.95})
	v := a.View("p1")
	if v.Observations != 1 {
		t.Errorf("after prune obs count = %d, want 1", v.Observations)
	}
	if _, ok := v.ByVector[VectorBLE]; ok {
		t.Errorf("stale BLE obs survived prune")
	}
}

func TestAggregatorAllPeersSortedByTrust(t *testing.T) {
	a := NewAggregator()
	now := int64(20_000_000)
	a.NowFunc = func() int64 { return now }
	a.Record(PeerObservation{Vector: VectorBLE, PeerID: "p1", ObservedAtMs: now, RSSI: -85})
	a.Record(PeerObservation{Vector: VectorBLE, PeerID: "p2", ObservedAtMs: now, RSSI: -50})
	a.Record(PeerObservation{Vector: VectorSonic, PeerID: "p2", ObservedAtMs: now, SignalQual: 0.95})
	a.Record(PeerObservation{Vector: VectorUWB, PeerID: "p3", ObservedAtMs: now, DistanceM: 0.5, SignalQual: 0.9})
	a.Record(PeerObservation{Vector: VectorSonic, PeerID: "p3", ObservedAtMs: now, SignalQual: 0.95})

	peers := a.AllPeers()
	if len(peers) != 3 {
		t.Fatalf("got %d peers, want 3", len(peers))
	}
	if peers[0].PeerID != "p3" || peers[2].PeerID != "p1" {
		t.Errorf("sort wrong: %+v", peers)
	}
}

func TestAggregatorIgnoresUnknownVector(t *testing.T) {
	a := NewAggregator()
	a.Record(PeerObservation{Vector: VectorID("zombie"), PeerID: "p1", ObservedAtMs: 1, RSSI: -50})
	if v := a.TrustOf("p1"); v != 0 {
		t.Errorf("unknown vector trusted = %v, want 0", v)
	}
}

func TestFallbackOfEmptyChainDefault(t *testing.T) {
	// Nothing registered → degenerate chain [primary]
	chain := FallbackOf(VectorID("never-registered"), "")
	if len(chain) != 1 || chain[0] != VectorID("never-registered") {
		t.Errorf("default fallback chain wrong: %v", chain)
	}
}

func TestPlatformSetGet(t *testing.T) {
	SetPlatform("ios")
	if Platform() != "ios" {
		t.Errorf("Platform() = %q, want ios", Platform())
	}
	SetPlatform("")
	if Platform() != "" {
		t.Errorf("Platform() = %q, want empty", Platform())
	}
}
