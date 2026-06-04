package sonic

import (
	"bytes"
	"testing"

	"github.com/ericdequ/waves_worx/go/transport"
)

func TestFrameRoundtrip(t *testing.T) {
	in := Payload{
		Kind:       KindCohort,
		TimeBucket: 0xCAFE,
		Nonce:      []byte("01234567"),
		PeerTag:    []byte{0xAA, 0xBB, 0xCC},
	}
	frame, err := EncodeFrame(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != FrameLen {
		t.Fatalf("frame len = %d, want %d", len(frame), FrameLen)
	}
	out, err := DecodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind != in.Kind || out.TimeBucket != in.TimeBucket {
		t.Errorf("roundtrip mismatch")
	}
	if !bytes.Equal(out.Nonce, in.Nonce) || !bytes.Equal(out.PeerTag, in.PeerTag) {
		t.Errorf("byte fields mismatch")
	}
}

func TestCohortNonceDeterministic(t *testing.T) {
	secret := []byte("lobby-secret-v1")
	t1 := int64(1715000000)
	if !bytes.Equal(CohortNonce(secret, t1), CohortNonce(secret, t1)) {
		t.Errorf("cohort nonce not deterministic")
	}
	if bytes.Equal(CohortNonce(secret, t1), CohortNonce(secret, t1+1)) {
		t.Errorf("cohort nonce identical across seconds — broken schedule")
	}
}

func TestCohortNonceDifferentSecretsDiffer(t *testing.T) {
	t1 := int64(1715000000)
	if bytes.Equal(CohortNonce([]byte("a"), t1), CohortNonce([]byte("b"), t1)) {
		t.Errorf("different secrets produced identical nonce")
	}
}

func TestVerifyCohortNonceWithinTolerance(t *testing.T) {
	secret := []byte("lobby-secret-current")
	t1 := int64(1715000050)
	for _, delta := range []int{-2, -1, 0, 1, 2} {
		observed := CohortNonce(secret, t1+int64(delta))
		if !VerifyCohortNonce(secret, t1, 2, observed) {
			t.Errorf("verify failed for delta=%d within tolerance", delta)
		}
	}
}

func TestVerifyCohortNonceOutOfToleranceFails(t *testing.T) {
	secret := []byte("lobby-secret-v3")
	t1 := int64(1715000100)
	if VerifyCohortNonce(secret, t1, 2, CohortNonce(secret, t1+10)) {
		t.Errorf("accepted nonce 10s out of band — replay window too wide")
	}
}

func TestFallbackChainsRegistered(t *testing.T) {
	any := transport.FallbackOf(transport.VectorSonic, "")
	web := transport.FallbackOf(transport.VectorSonic, "web")
	ios := transport.FallbackOf(transport.VectorSonic, "ios")

	if len(any) == 0 || any[0] != transport.VectorSonic {
		t.Errorf("any-platform chain wrong: %v", any)
	}
	if len(web) != 1 || web[0] != transport.VectorBLE {
		t.Errorf("web chain wrong: %v", web)
	}
	if len(ios) < 2 || ios[1] != transport.VectorUWB {
		t.Errorf("ios chain wrong: %v", ios)
	}
}
