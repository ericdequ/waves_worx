package nfc

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/ericdequ/waves_worx/go/transport"
)

func TestHandshakeRoundtrip(t *testing.T) {
	pub := make([]byte, 32)
	_, _ = rand.Read(pub)
	sig := make([]byte, 64)
	_, _ = rand.Read(sig)
	in := Payload{Flags: 0x0001, PublicKey: pub, MintedAt: 1715000000, Signature: sig}
	frame, err := EncodeHandshake(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != HandshakeLen {
		t.Fatalf("frame len = %d, want %d", len(frame), HandshakeLen)
	}
	out, err := DecodeHandshake(frame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.PublicKey, pub) || !bytes.Equal(out.Signature, sig) {
		t.Errorf("byte fields didn't roundtrip")
	}
	if out.MintedAt != in.MintedAt || out.Flags != in.Flags {
		t.Errorf("scalar fields didn't roundtrip")
	}
	sb, err := SigningPayload(pub, in.MintedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(sb) != 40 {
		t.Errorf("signing payload len = %d, want 40", len(sb))
	}
}

func TestRejectsBadLength(t *testing.T) {
	if _, err := DecodeHandshake(make([]byte, 50)); err == nil {
		t.Errorf("expected length error")
	}
}

func TestFallbackChainsRegistered(t *testing.T) {
	// iOS chain must NOT lead with NFC — that's the whole point of the
	// platform split. NFC peer is dead on iOS since iOS 13.
	ios := transport.FallbackOf(transport.VectorNFC, "ios")
	if len(ios) == 0 || ios[0] == transport.VectorNFC {
		t.Errorf("iOS NFC fallback leads with NFC — that's broken: %v", ios)
	}
	// Android chain SHOULD lead with NFC.
	android := transport.FallbackOf(transport.VectorNFC, "android")
	if len(android) == 0 || android[0] != transport.VectorNFC {
		t.Errorf("Android NFC fallback should lead with NFC: %v", android)
	}
}
