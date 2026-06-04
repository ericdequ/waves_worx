// Package nfc carries BEV's NFC NDEF handshake record format.
//
// **Reality check:** peer-to-peer NFC was removed from iOS in iOS 13. This
// transport is **Android↔Android only**. The Go encoder is platform-
// agnostic, but the radio side only works on Android. iOS fallback chain
// routes around NFC entirely.
//
// NDEF external-type record: "application/vnd.bev.handshake.v1"
//
// Payload (108 bytes, fits in any NDEF short record):
//
//	[0..3]    protocol version + flags (uint32 BE)
//	[4..35]   peer Ed25519 public key (32 bytes)
//	[36..43]  minted-at unix seconds (uint64 BE)
//	[44..107] signature of (pubkey || mintedAt) — 64 bytes, Ed25519
//
// Fallback chain (registered at init):
//
//	android: [nfc, uwb, sonic]      — NFC works; UWB on Android 14+ as 2nd
//	ios:     [uwb, sonic, ble]      — NFC peer dead; UWB tap on capable iPhones
//	web:     [sonic]                — no NFC on web
package nfc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ericdequ/waves_worx/go/transport"
)

const (
	RecordType       = "application/vnd.bev.handshake.v1"
	handshakeVersion = 1
	HandshakeLen     = 108
	pubKeyLen        = 32
	signatureLen     = 64
)

func init() {
	transport.RegisterFallback(transport.VectorNFC, "android", transport.FallbackChain{
		transport.VectorNFC,
		transport.VectorUWB,
		transport.VectorSonic,
	})
	transport.RegisterFallback(transport.VectorNFC, "ios", transport.FallbackChain{
		transport.VectorUWB,
		transport.VectorSonic,
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorNFC, "web", transport.FallbackChain{
		transport.VectorSonic,
	})
}

// Payload is the parsed view of a tap-handshake record.
type Payload struct {
	Version   int    `json:"version"`
	Flags     uint16 `json:"flags"`
	PublicKey []byte `json:"publicKey"` // 32 bytes
	MintedAt  int64  `json:"mintedAt"`
	Signature []byte `json:"signature"` // 64 bytes
}

// EncodeHandshake packs a Payload into its 108-byte NDEF payload form.
// The signature must already be computed by the caller — Go's middle-end
// doesn't hold private keys.
func EncodeHandshake(p Payload) ([]byte, error) {
	if len(p.PublicKey) != pubKeyLen {
		return nil, fmt.Errorf("publicKey must be %d bytes, got %d", pubKeyLen, len(p.PublicKey))
	}
	if len(p.Signature) != signatureLen {
		return nil, fmt.Errorf("signature must be %d bytes, got %d", signatureLen, len(p.Signature))
	}
	buf := make([]byte, HandshakeLen)
	binary.BigEndian.PutUint16(buf[0:2], uint16(handshakeVersion))
	binary.BigEndian.PutUint16(buf[2:4], p.Flags)
	copy(buf[4:36], p.PublicKey)
	binary.BigEndian.PutUint64(buf[36:44], uint64(p.MintedAt))
	copy(buf[44:108], p.Signature)
	return buf, nil
}

// DecodeHandshake parses a 108-byte payload back into Payload. Does NOT
// verify the signature — Ed25519 verify is the caller's job after they
// pull out PublicKey + SigningPayload.
func DecodeHandshake(buf []byte) (Payload, error) {
	if len(buf) != HandshakeLen {
		return Payload{}, fmt.Errorf("nfc handshake must be %d bytes, got %d", HandshakeLen, len(buf))
	}
	version := int(binary.BigEndian.Uint16(buf[0:2]))
	if version != handshakeVersion {
		return Payload{}, fmt.Errorf("nfc handshake version mismatch: got %d, want %d", version, handshakeVersion)
	}
	mintedAt := int64(binary.BigEndian.Uint64(buf[36:44]))
	if mintedAt <= 0 {
		return Payload{}, errors.New("nfc handshake mintedAt must be positive")
	}
	return Payload{
		Version:   version,
		Flags:     binary.BigEndian.Uint16(buf[2:4]),
		PublicKey: append([]byte(nil), buf[4:36]...),
		MintedAt:  mintedAt,
		Signature: append([]byte(nil), buf[44:108]...),
	}, nil
}

// SigningPayload returns the bytes that should be Ed25519-signed to produce
// Payload.Signature. Caller (native side) computes the signature using
// the device's keychain identity key.
func SigningPayload(publicKey []byte, mintedAt int64) ([]byte, error) {
	if len(publicKey) != pubKeyLen {
		return nil, fmt.Errorf("publicKey must be %d bytes, got %d", pubKeyLen, len(publicKey))
	}
	out := make([]byte, pubKeyLen+8)
	copy(out, publicKey)
	binary.BigEndian.PutUint64(out[pubKeyLen:], uint64(mintedAt))
	return out, nil
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type encodeInput struct {
	Flags     uint16 `json:"flags"`
	PublicKey []byte `json:"publicKey"`
	MintedAt  int64  `json:"mintedAt"`
	Signature []byte `json:"signature"`
}

type encodeResult struct {
	Payload []byte `json:"payload"`
}

func ApplyEncode(payload string) (encodeResult, error) {
	var in encodeInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return encodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	out, err := EncodeHandshake(Payload{
		Flags:     in.Flags,
		PublicKey: in.PublicKey,
		MintedAt:  in.MintedAt,
		Signature: in.Signature,
	})
	if err != nil {
		return encodeResult{}, err
	}
	return encodeResult{Payload: out}, nil
}

type decodeResult struct {
	Payload        Payload `json:"payload"`
	SigningPayload []byte  `json:"signingPayload"`
}

func ApplyDecode(payload string) (decodeResult, error) {
	var in struct {
		Payload []byte `json:"payload"`
	}
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	p, err := DecodeHandshake(in.Payload)
	if err != nil {
		return decodeResult{}, err
	}
	sigBytes, _ := SigningPayload(p.PublicKey, p.MintedAt)
	return decodeResult{Payload: p, SigningPayload: sigBytes}, nil
}
