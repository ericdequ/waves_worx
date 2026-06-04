package sonic

import (
	"bytes"
	"testing"
)

func TestDeriveChannelKeyDeterministic(t *testing.T) {
	secret := []byte("lobby-secret")
	k1 := DeriveChannelKey(secret, "groupA")
	k2 := DeriveChannelKey(secret, "groupA")
	if !bytes.Equal(k1, k2) {
		t.Errorf("derive not deterministic for same inputs")
	}
	if len(k1) != 32 {
		t.Errorf("key length = %d, want 32 (AES-256)", len(k1))
	}
}

func TestDeriveChannelKeyDifferentChannels(t *testing.T) {
	secret := []byte("lobby-secret")
	if bytes.Equal(DeriveChannelKey(secret, "groupA"), DeriveChannelKey(secret, "groupB")) {
		t.Errorf("different channels produced same key — keys leak across lanes")
	}
}

func TestDeriveChannelKeyDifferentSecrets(t *testing.T) {
	a := DeriveChannelKey([]byte("aaa"), "groupA")
	b := DeriveChannelKey([]byte("bbb"), "groupA")
	if bytes.Equal(a, b) {
		t.Errorf("different secrets produced same key — channel key leaky")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := DeriveChannelKey([]byte("secret"), "groupA")
	plain := []byte("hello bev private lane")
	ct, err := EncryptFrame(key, plain, 42)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptFrame(key, ct, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("plaintext roundtrip mismatch")
	}
}

func TestDecryptWithWrongCounterFails(t *testing.T) {
	key := DeriveChannelKey([]byte("secret"), "groupA")
	ct, _ := EncryptFrame(key, []byte("payload"), 1)
	if _, err := DecryptFrame(key, ct, 2); err == nil {
		t.Errorf("decrypt accepted wrong counter — replay-protection broken")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	keyA := DeriveChannelKey([]byte("secret"), "groupA")
	keyB := DeriveChannelKey([]byte("secret"), "groupB")
	ct, _ := EncryptFrame(keyA, []byte("payload"), 1)
	if _, err := DecryptFrame(keyB, ct, 1); err == nil {
		t.Errorf("decrypt accepted wrong key — channel isolation broken")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	key := DeriveChannelKey([]byte("secret"), "groupA")
	ct, _ := EncryptFrame(key, []byte("payload"), 1)
	ct[5] ^= 0x01 // flip a byte
	if _, err := DecryptFrame(key, ct, 1); err == nil {
		t.Errorf("decrypt accepted tampered ciphertext — auth tag broken")
	}
}

func TestEncryptRejectsBadKeyLen(t *testing.T) {
	if _, err := EncryptFrame([]byte("short"), []byte("x"), 0); err == nil {
		t.Errorf("expected error for short key")
	}
}

func TestNonceCounterUniqueness(t *testing.T) {
	key := DeriveChannelKey([]byte("secret"), "groupA")
	n1 := nonceFor(key, 1)
	n2 := nonceFor(key, 2)
	if bytes.Equal(n1, n2) {
		t.Errorf("same key + different counter produced same nonce — catastrophic")
	}
	if len(n1) != 12 {
		t.Errorf("nonce length = %d, want 12 (GCM)", len(n1))
	}
}
