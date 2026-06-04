package uwb

import (
	"testing"
)

func TestLatestStoresAndReturns(t *testing.T) {
	ClearLatest()
	o := Observation{
		PeerID:       "peer-uwb-1",
		DistanceM:    2.3,
		AzimuthDeg:   45.0,
		ElevationDeg: -10.0,
		Confidence:   0.92,
		ObservedAtMs: 1_000_000,
	}
	storeLatest(o)
	got, ok := Latest("peer-uwb-1")
	if !ok {
		t.Fatal("Latest returned not-ok for stored observation")
	}
	if got.DistanceM != o.DistanceM || got.AzimuthDeg != o.AzimuthDeg {
		t.Errorf("Latest roundtrip mismatch: got %+v", got)
	}
}

func TestLatestIgnoresOutOfOrder(t *testing.T) {
	ClearLatest()
	storeLatest(Observation{PeerID: "p", DistanceM: 1.0, ObservedAtMs: 2_000})
	storeLatest(Observation{PeerID: "p", DistanceM: 99.0, ObservedAtMs: 1_000}) // older
	got, _ := Latest("p")
	if got.DistanceM != 1.0 {
		t.Errorf("out-of-order observation overwrote newer: got dist=%v", got.DistanceM)
	}
}

func TestLatestMissingPeer(t *testing.T) {
	ClearLatest()
	if _, ok := Latest("never-seen"); ok {
		t.Errorf("Latest returned ok for unknown peer")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		dist, conf float64
		want       MeetupVerdict
	}{
		{0.5, 0.9, MeetupHandshake},
		{1.0, 0.9, MeetupHandshake},
		{2.0, 0.9, MeetupNearby},
		{6.0, 0.9, MeetupSameRoom},
		{15.0, 0.9, MeetupDistant},
		{0.5, 0.3, MeetupNearby},   // low conf demotes
		{2.0, 0.3, MeetupSameRoom}, // low conf demotes
	}
	for _, c := range cases {
		got := Classify(Observation{DistanceM: c.dist, Confidence: c.conf})
		if got != c.want {
			t.Errorf("dist=%v conf=%v: got %s, want %s", c.dist, c.conf, got, c.want)
		}
	}
}
