// Package ble carries BEV's BLE manufacturer-data frame format + the
// Apply ops that pack/unpack it. Pairs with the native BLE plugin
// (vendor/capacitor-go-core/ios + android) which owns the radio.
//
// Frame (21 bytes, fits in BLE 4.2 manufacturer-data field):
//
//	[0]      version (currently 1)
//	[1]      kind: 1=vibe, 2=identity, 3=gossip-tease
//	[2..8]   geohash7 (7 ASCII bytes; "0000000" if unknown)
//	[9..10]  vibe score (uint16 BE — tier-derived; 0 if unset)
//	[11..14] rotating ephemeral peer ID (4 bytes)
//	[15..16] HMAC truncation (2 bytes; replay defense)
//	[17..19] reserved (must be zero)
//	[20]     XOR checksum of [0..19]
//
// Fallback chain (registered at init):
//
//	any platform: [ble, sonic, wifi_p2p]
//
// BLE is the most universal vector. If it's unavailable (rare — every modern
// phone has BLE), sonic is the next-best universal fallback, then Wi-Fi P2P
// for higher bandwidth.
package ble

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ericdequ/waves_worx/go/transport"
	"github.com/ericdequ/waves_worx/go/transport/internal/codec"
)

const (
	frameVersion = 1
	FrameLen     = 21

	KindVibe        byte = 1
	KindIdentity    byte = 2
	KindGossipTease byte = 3
)

func init() {
	transport.RegisterFallback(transport.VectorBLE, "", transport.FallbackChain{
		transport.VectorBLE,
		transport.VectorSonic,
		transport.VectorWiFiP2P,
	})
}

// Payload is the user-data view of a BLE frame.
type Payload struct {
	Version     int    `json:"version"`
	Kind        byte   `json:"kind"`
	Geohash7    string `json:"geohash7"`
	VibeScore   uint16 `json:"vibeScore"`
	EphemeralID []byte `json:"ephemeralId"` // 4 bytes
	HMACTag     []byte `json:"hmacTag"`     // 2 bytes
}

// EncodeFrame packs a Payload into the 21-byte wire format. If hmacKey is
// non-nil, an HMAC-SHA256 of bytes [0..14] is truncated to 2 bytes and
// written to [15..16]; otherwise p.HMACTag is used verbatim.
//
// Returns an error on any malformed input — frame layout is a contract,
// callers shouldn't get silent padding.
func EncodeFrame(p Payload, hmacKey []byte) ([]byte, error) {
	if len(p.EphemeralID) != 4 {
		return nil, fmt.Errorf("ephemeralId must be 4 bytes, got %d", len(p.EphemeralID))
	}
	if len(p.Geohash7) != 7 {
		return nil, fmt.Errorf("geohash7 must be 7 chars, got %d", len(p.Geohash7))
	}
	for i, c := range p.Geohash7 {
		if c == '0' { // "0000000" sentinel = "no location"
			continue
		}
		if !codec.IsGeohashChar(byte(c)) {
			return nil, fmt.Errorf("geohash7 has invalid char %q at %d", c, i)
		}
	}

	buf := make([]byte, FrameLen)
	buf[0] = frameVersion
	buf[1] = p.Kind
	copy(buf[2:9], []byte(p.Geohash7))
	binary.BigEndian.PutUint16(buf[9:11], p.VibeScore)
	copy(buf[11:15], p.EphemeralID)

	if hmacKey != nil {
		mac := hmac.New(sha256.New, hmacKey)
		mac.Write(buf[:15])
		sum := mac.Sum(nil)
		copy(buf[15:17], sum[:2])
	} else {
		if len(p.HMACTag) != 2 {
			return nil, fmt.Errorf("hmacTag must be 2 bytes when hmacKey nil, got %d", len(p.HMACTag))
		}
		copy(buf[15:17], p.HMACTag)
	}
	buf[20] = codec.XORChecksum(buf[:20])
	return buf, nil
}

// DecodeFrame parses a 21-byte wire frame back into a Payload. Returns an
// error on length/checksum/version failure. HMAC verification is separate
// (caller has the key).
func DecodeFrame(buf []byte) (Payload, error) {
	if len(buf) != FrameLen {
		return Payload{}, fmt.Errorf("ble frame must be %d bytes, got %d", FrameLen, len(buf))
	}
	if codec.XORChecksum(buf[:20]) != buf[20] {
		return Payload{}, errors.New("ble frame checksum mismatch")
	}
	if buf[0] != frameVersion {
		return Payload{}, fmt.Errorf("ble frame version mismatch: got %d, want %d", buf[0], frameVersion)
	}
	return Payload{
		Version:     int(buf[0]),
		Kind:        buf[1],
		Geohash7:    string(buf[2:9]),
		VibeScore:   binary.BigEndian.Uint16(buf[9:11]),
		EphemeralID: append([]byte(nil), buf[11:15]...),
		HMACTag:     append([]byte(nil), buf[15:17]...),
	}, nil
}

// VerifyHMAC checks the 2-byte HMAC truncation against the supplied key.
// Constant-time comparison.
func VerifyHMAC(buf []byte, hmacKey []byte) bool {
	if len(buf) != FrameLen || len(hmacKey) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(buf[:15])
	sum := mac.Sum(nil)
	return hmac.Equal(sum[:2], buf[15:17])
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type encodeInput struct {
	Kind        byte   `json:"kind"`
	Geohash7    string `json:"geohash7"`
	VibeScore   uint16 `json:"vibeScore"`
	EphemeralID []byte `json:"ephemeralId"`
	HMACKey     []byte `json:"hmacKey,omitempty"`
	HMACTag     []byte `json:"hmacTag,omitempty"`
}

type encodeResult struct {
	Frame []byte `json:"frame"`
}

func ApplyEncode(payload string) (encodeResult, error) {
	var in encodeInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return encodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	frame, err := EncodeFrame(Payload{
		Kind:        in.Kind,
		Geohash7:    in.Geohash7,
		VibeScore:   in.VibeScore,
		EphemeralID: in.EphemeralID,
		HMACTag:     in.HMACTag,
	}, in.HMACKey)
	if err != nil {
		return encodeResult{}, err
	}
	return encodeResult{Frame: frame}, nil
}

type decodeInput struct {
	Frame   []byte `json:"frame"`
	HMACKey []byte `json:"hmacKey,omitempty"`
}

type decodeResult struct {
	Payload   Payload `json:"payload"`
	HMACValid bool    `json:"hmacValid"`
}

func ApplyDecode(payload string) (decodeResult, error) {
	var in decodeInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	p, err := DecodeFrame(in.Frame)
	if err != nil {
		return decodeResult{}, err
	}
	res := decodeResult{Payload: p}
	if len(in.HMACKey) > 0 {
		res.HMACValid = VerifyHMAC(in.Frame, in.HMACKey)
	}
	return res, nil
}
