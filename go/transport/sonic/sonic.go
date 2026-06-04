// Package sonic carries BEV's ultrasonic (18-20 kHz) frame format + the
// cohort HMAC schedule for the synchronized-cohort emission feature.
// Pairs with a future native sonic plugin built on ggwave (Apache 2.0).
//
// Frame (16 bytes — ggwave normal-mode payload budget):
//
//	[0]      version (1)
//	[1]      kind: 1=cohort chirp, 2=identity beacon, 3=handshake
//	[2..3]   cohort time bucket (uint16 LE, seconds % 65536)
//	[4..11]  HMAC-derived nonce (8 bytes)
//	[12..14] peer tag (3 bytes — short ephemeral)
//	[15]     XOR checksum of [0..14]
//
// Cohort schedule:
//
//	nonce(t) = HMAC-SHA256(lobby_secret, "cohort\0" || uint64BE(t))[:8]
//
// All synchronized emitters in the same lobby produce identical nonces for
// the same second; an impostor without lobby_secret cannot predict the next
// frame. Cloud (RTDB) distributes lobby_secret at join.
//
// Fallback chain (registered at init):
//
//	any:     [sonic, ble, wifi_p2p]
//	web:     [ble]    (no native sonic emitter on web — Web Audio bg restrictions)
//	ios:     [sonic, uwb, ble]  (UWB for same-room proof on capable iPhones)
package sonic

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
	FrameLen     = 16

	KindCohort    byte = 1
	KindIdentity  byte = 2
	KindHandshake byte = 3

	cohortContextLabel = "cohort\x00"
)

func init() {
	transport.RegisterFallback(transport.VectorSonic, "", transport.FallbackChain{
		transport.VectorSonic,
		transport.VectorBLE,
		transport.VectorWiFiP2P,
	})
	transport.RegisterFallback(transport.VectorSonic, "web", transport.FallbackChain{
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorSonic, "ios", transport.FallbackChain{
		transport.VectorSonic,
		transport.VectorUWB,
		transport.VectorBLE,
	})
}

// Payload is the user-data view of a sonic frame.
type Payload struct {
	Version    int    `json:"version"`
	Kind       byte   `json:"kind"`
	TimeBucket uint16 `json:"timeBucket"`
	Nonce      []byte `json:"nonce"`   // 8 bytes
	PeerTag    []byte `json:"peerTag"` // 3 bytes
}

// EncodeFrame packs a Payload into the 16-byte wire format.
func EncodeFrame(p Payload) ([]byte, error) {
	if len(p.Nonce) != 8 {
		return nil, fmt.Errorf("nonce must be 8 bytes, got %d", len(p.Nonce))
	}
	if len(p.PeerTag) != 3 {
		return nil, fmt.Errorf("peerTag must be 3 bytes, got %d", len(p.PeerTag))
	}
	buf := make([]byte, FrameLen)
	buf[0] = frameVersion
	buf[1] = p.Kind
	binary.LittleEndian.PutUint16(buf[2:4], p.TimeBucket)
	copy(buf[4:12], p.Nonce)
	copy(buf[12:15], p.PeerTag)
	buf[15] = codec.XORChecksum(buf[:15])
	return buf, nil
}

// DecodeFrame parses a 16-byte wire frame back into a Payload.
func DecodeFrame(buf []byte) (Payload, error) {
	if len(buf) != FrameLen {
		return Payload{}, fmt.Errorf("sonic frame must be %d bytes, got %d", FrameLen, len(buf))
	}
	if codec.XORChecksum(buf[:15]) != buf[15] {
		return Payload{}, errors.New("sonic frame checksum mismatch")
	}
	if buf[0] != frameVersion {
		return Payload{}, fmt.Errorf("sonic frame version mismatch: got %d, want %d", buf[0], frameVersion)
	}
	return Payload{
		Version:    int(buf[0]),
		Kind:       buf[1],
		TimeBucket: binary.LittleEndian.Uint16(buf[2:4]),
		Nonce:      append([]byte(nil), buf[4:12]...),
		PeerTag:    append([]byte(nil), buf[12:15]...),
	}, nil
}

// CohortNonce derives the 8-byte HMAC nonce for the given lobby secret and
// epoch second. Deterministic; identical inputs → identical output across
// every phone in the cohort. Load-bearing primitive of synchronized
// cohort emission (see ULTRASONIC_VIBE.md).
func CohortNonce(lobbySecret []byte, epochSec int64) []byte {
	if len(lobbySecret) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, lobbySecret)
	mac.Write([]byte(cohortContextLabel))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(epochSec))
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	out := make([]byte, 8)
	copy(out, sum[:8])
	return out
}

// VerifyCohortNonce returns true if observed matches the HMAC nonce for
// any second within toleranceSec of expectedEpochSec. Allows for small
// clock skew between phones (Wi-Fi-synced clocks are typically <1s; we
// default to ±2s at the call site).
func VerifyCohortNonce(lobbySecret []byte, expectedEpochSec int64, toleranceSec int, observed []byte) bool {
	if len(observed) != 8 {
		return false
	}
	for delta := -toleranceSec; delta <= toleranceSec; delta++ {
		exp := CohortNonce(lobbySecret, expectedEpochSec+int64(delta))
		if hmac.Equal(exp, observed) {
			return true
		}
	}
	return false
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type encodeInput struct {
	Kind       byte   `json:"kind"`
	TimeBucket uint16 `json:"timeBucket"`
	Nonce      []byte `json:"nonce"`
	PeerTag    []byte `json:"peerTag"`
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
		Kind:       in.Kind,
		TimeBucket: in.TimeBucket,
		Nonce:      in.Nonce,
		PeerTag:    in.PeerTag,
	})
	if err != nil {
		return encodeResult{}, err
	}
	return encodeResult{Frame: frame}, nil
}

type decodeResult struct {
	Payload Payload `json:"payload"`
}

func ApplyDecode(payload string) (decodeResult, error) {
	var in struct {
		Frame []byte `json:"frame"`
	}
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	p, err := DecodeFrame(in.Frame)
	if err != nil {
		return decodeResult{}, err
	}
	return decodeResult{Payload: p}, nil
}

type cohortScheduleInput struct {
	LobbySecret []byte `json:"lobbySecret"`
	EpochSec    int64  `json:"epochSec"`
}

type cohortScheduleResult struct {
	Nonce []byte `json:"nonce"`
}

func ApplyCohortSchedule(payload string) (cohortScheduleResult, error) {
	var in cohortScheduleInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return cohortScheduleResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	nonce := CohortNonce(in.LobbySecret, in.EpochSec)
	if nonce == nil {
		return cohortScheduleResult{}, errors.New("lobbySecret required")
	}
	return cohortScheduleResult{Nonce: nonce}, nil
}

type cohortVerifyInput struct {
	LobbySecret  []byte `json:"lobbySecret"`
	EpochSec     int64  `json:"epochSec"`
	ToleranceSec int    `json:"toleranceSec,omitempty"`
	Observed     []byte `json:"observed"`
}

type cohortVerifyResult struct {
	Valid bool `json:"valid"`
}

func ApplyCohortVerify(payload string) (cohortVerifyResult, error) {
	var in cohortVerifyInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return cohortVerifyResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	tol := in.ToleranceSec
	if tol <= 0 {
		tol = 2
	}
	return cohortVerifyResult{
		Valid: VerifyCohortNonce(in.LobbySecret, in.EpochSec, tol, in.Observed),
	}, nil
}
