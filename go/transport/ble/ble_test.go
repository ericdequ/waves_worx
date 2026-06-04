package ble

import (
	"bytes"
	"testing"

	"github.com/ericdequ/waves_worx/go/transport"
)

func TestFrameRoundtripWithKey(t *testing.T) {
	key := []byte("test-hmac-key-1")
	in := Payload{
		Kind:        KindVibe,
		Geohash7:    "9q8yyk8",
		VibeScore:   42,
		EphemeralID: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	frame, err := EncodeFrame(in, key)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(frame) != FrameLen {
		t.Fatalf("frame len = %d, want %d", len(frame), FrameLen)
	}
	if !VerifyHMAC(frame, key) {
		t.Fatalf("hmac verify failed on freshly-encoded frame")
	}
	out, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Kind != in.Kind || out.Geohash7 != in.Geohash7 || out.VibeScore != in.VibeScore {
		t.Errorf("roundtrip mismatch")
	}
	if !bytes.Equal(out.EphemeralID, in.EphemeralID) {
		t.Errorf("ephemeral id mismatch")
	}
}

func TestWrongKeyHMACFails(t *testing.T) {
	frame, err := EncodeFrame(Payload{
		Kind:        KindIdentity,
		Geohash7:    "0000000",
		EphemeralID: []byte{1, 2, 3, 4},
	}, []byte("good"))
	if err != nil {
		t.Fatal(err)
	}
	if VerifyHMAC(frame, []byte("bad")) {
		t.Errorf("hmac verified with wrong key — that's a leak")
	}
}

func TestTamperedChecksumRejected(t *testing.T) {
	frame, _ := EncodeFrame(Payload{
		Kind:        KindVibe,
		Geohash7:    "9q8yyk8",
		EphemeralID: []byte{0, 0, 0, 0},
	}, []byte("k"))
	frame[5] ^= 0x01
	if _, err := DecodeFrame(frame); err == nil {
		t.Errorf("expected checksum failure after tamper")
	}
}

func TestRejectsBadGeohash(t *testing.T) {
	_, err := EncodeFrame(Payload{
		Kind:        KindVibe,
		Geohash7:    "9q8aaaa", // 'a' not in BEV base32
		EphemeralID: []byte{0, 0, 0, 0},
	}, []byte("k"))
	if err == nil {
		t.Errorf("expected error for invalid geohash char")
	}
}

func TestRejectsBadEphemeralLen(t *testing.T) {
	_, err := EncodeFrame(Payload{
		Kind:        KindVibe,
		Geohash7:    "9q8yyk8",
		EphemeralID: []byte{1, 2, 3},
	}, []byte("k"))
	if err == nil {
		t.Errorf("expected error for wrong ephemeral length")
	}
}

func TestFallbackChainRegistered(t *testing.T) {
	chain := transport.FallbackOf(transport.VectorBLE, "")
	if len(chain) == 0 || chain[0] != transport.VectorBLE {
		t.Errorf("BLE fallback chain not registered or wrong primary: %v", chain)
	}
}
