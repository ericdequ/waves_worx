package wifi

import "testing"

func TestEncodeDecodeRoundtrip(t *testing.T) {
	in := Envelope{Kind: "identity", PeerID: "peer-abc", Geohash9: "9q8yyk8yp", IssuedAt: 1715000000}
	buf, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decode(buf)
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind != in.Kind || out.PeerID != in.PeerID || out.Geohash9 != in.Geohash9 {
		t.Errorf("envelope roundtrip mismatch")
	}
	if out.Version != 1 {
		t.Errorf("version default not applied: got %d", out.Version)
	}
}

func TestEncodeRejectsMissingPeerID(t *testing.T) {
	if _, err := Encode(Envelope{Kind: "x"}); err == nil {
		t.Errorf("expected error for missing peerId")
	}
}
