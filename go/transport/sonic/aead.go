package sonic

// Per-channel AEAD encryption for private sonic lanes. AES-256-GCM with
// keys derived from a shared lobby secret + the channel ID via HMAC-SHA256.
//
// The nonce comes from a counter derived from the cohort time bucket +
// caller-supplied frame counter — every phone in the lobby on the same
// channel computes the same nonce for the same frame without needing
// synchronization beyond the lobby_secret + the cohort schedule.
//
// AES-GCM was chosen over ChaCha20-Poly1305 because it's in the Go stdlib
// (crypto/aes + crypto/cipher) — no extra deps. Both are equivalent AEAD
// constructions; AES-NI hardware acceleration is present on every device
// we care about.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	channelKeyContext   = "sonic-channel\x00"
	channelNonceContext = "nonce\x00"
)

// DeriveChannelKey produces the 32-byte AES-256 key for (lobbySecret, channelID).
// Deterministic — same inputs always yield same key. Empty inputs return nil.
func DeriveChannelKey(lobbySecret []byte, channelID string) []byte {
	if len(lobbySecret) == 0 || channelID == "" {
		return nil
	}
	mac := hmac.New(sha256.New, lobbySecret)
	mac.Write([]byte(channelKeyContext))
	mac.Write([]byte(channelID))
	return mac.Sum(nil) // SHA-256 = 32 bytes — exactly AES-256 key size
}

// nonceFor derives the 12-byte AES-GCM nonce for a given frame counter.
// Counter must be unique per (key, frame); reusing a (key, counter) pair
// catastrophically breaks GCM. Use the cohort schedule's per-second
// counter or a monotonic per-session counter.
func nonceFor(channelKey []byte, counter uint64) []byte {
	mac := hmac.New(sha256.New, channelKey)
	mac.Write([]byte(channelNonceContext))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	out := make([]byte, 12)
	copy(out, sum[:12])
	return out
}

// EncryptFrame encrypts plaintext with the channel key for the given counter.
// The output includes the 16-byte GCM auth tag appended to the ciphertext.
// Decryption side will reject any tampered output.
func EncryptFrame(channelKey, plaintext []byte, counter uint64) ([]byte, error) {
	if len(channelKey) != 32 {
		return nil, errors.New("channelKey must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(channelKey)
	if err != nil {
		return nil, fmt.Errorf("aes init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm init: %w", err)
	}
	nonce := nonceFor(channelKey, counter)
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// DecryptFrame validates + decrypts ciphertext with the channel key for
// the given counter. Returns an error on any tamper detection (auth-tag
// mismatch) or wrong key.
func DecryptFrame(channelKey, ciphertext []byte, counter uint64) ([]byte, error) {
	if len(channelKey) != 32 {
		return nil, errors.New("channelKey must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(channelKey)
	if err != nil {
		return nil, fmt.Errorf("aes init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm init: %w", err)
	}
	nonce := nonceFor(channelKey, counter)
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("aead open: %w", err)
	}
	return plain, nil
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type deriveKeyInput struct {
	LobbySecret []byte `json:"lobbySecret"`
	ChannelID   string `json:"channelId"`
}

type deriveKeyResult struct {
	Key []byte `json:"key"` // 32 bytes
}

func ApplyDeriveChannelKey(payload string) (deriveKeyResult, error) {
	var in deriveKeyInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return deriveKeyResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	k := DeriveChannelKey(in.LobbySecret, in.ChannelID)
	if k == nil {
		return deriveKeyResult{}, errors.New("lobbySecret + channelId required")
	}
	return deriveKeyResult{Key: k}, nil
}

type encryptInput struct {
	ChannelKey []byte `json:"channelKey"`
	Plaintext  []byte `json:"plaintext"`
	Counter    uint64 `json:"counter"`
}

type encryptResult struct {
	Ciphertext []byte `json:"ciphertext"`
}

func ApplyEncryptChannel(payload string) (encryptResult, error) {
	var in encryptInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return encryptResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	ct, err := EncryptFrame(in.ChannelKey, in.Plaintext, in.Counter)
	if err != nil {
		return encryptResult{}, err
	}
	return encryptResult{Ciphertext: ct}, nil
}

type decryptInput struct {
	ChannelKey []byte `json:"channelKey"`
	Ciphertext []byte `json:"ciphertext"`
	Counter    uint64 `json:"counter"`
}

type decryptResult struct {
	Plaintext []byte `json:"plaintext"`
}

func ApplyDecryptChannel(payload string) (decryptResult, error) {
	var in decryptInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decryptResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	pt, err := DecryptFrame(in.ChannelKey, in.Ciphertext, in.Counter)
	if err != nil {
		return decryptResult{}, err
	}
	return decryptResult{Plaintext: pt}, nil
}
