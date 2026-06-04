// Package vlc encodes chunked screen-to-camera payloads. Production VLC
// needs hardware-level modulation; what we build here is a "QR-code-pulse"
// chunker that splits a payload into 60-byte frames the screen can render
// as time-sequenced patterns. ~1 kbps after error correction.
//
// Use case: "stack phones face-to-face to swap a small payload" — fun mode,
// not a primary transport. Each chunk renders for ~33ms at 30fps; receiver
// reads camera frame-by-frame and stitches by chunkIndex.
//
// Chunk layout (66 bytes):
//
//	[0..1]   chunk index (uint16 BE)
//	[2..3]   total chunks (uint16 BE)
//	[4..63]  chunk payload (60 bytes, zero-padded if last)
//	[64..65] CRC16 of [0..63]
//
// Fallback chain:
//
//	any:     [vlc, sonic, ble]    — VLC is rare; sonic is the next near-field option
//	web:     [sonic, ble]         — no native camera-loop on web for VLC decode
package vlc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/ericdequ/waves_worx/go/transport"
)

const (
	ChunkPayloadLen = 60
	chunkHeaderLen  = 4
	chunkTrailerLen = 2
	ChunkTotalLen   = chunkHeaderLen + ChunkPayloadLen + chunkTrailerLen
	maxChunks       = 1 << 16
)

func init() {
	transport.RegisterFallback(transport.VectorVLC, "", transport.FallbackChain{
		transport.VectorVLC,
		transport.VectorSonic,
		transport.VectorBLE,
	})
	transport.RegisterFallback(transport.VectorVLC, "web", transport.FallbackChain{
		transport.VectorSonic,
		transport.VectorBLE,
	})
}

// Encode splits a payload into chunks suitable for screen rendering. Returns
// error if the payload would exceed 65535 chunks (~3.9MB — past anything
// realistic for screen-to-camera).
func Encode(payload []byte) ([][]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("payload cannot be empty")
	}
	total := (len(payload) + ChunkPayloadLen - 1) / ChunkPayloadLen
	if total > maxChunks {
		return nil, fmt.Errorf("payload too large: %d chunks > %d", total, maxChunks)
	}
	out := make([][]byte, total)
	for i := 0; i < total; i++ {
		start := i * ChunkPayloadLen
		end := start + ChunkPayloadLen
		if end > len(payload) {
			end = len(payload)
		}
		buf := make([]byte, ChunkTotalLen)
		binary.BigEndian.PutUint16(buf[0:2], uint16(i))
		binary.BigEndian.PutUint16(buf[2:4], uint16(total))
		copy(buf[4:64], payload[start:end])
		crc := crc32.ChecksumIEEE(buf[:64])
		binary.BigEndian.PutUint16(buf[64:66], uint16(crc&0xFFFF))
		out[i] = buf
	}
	return out, nil
}

// Decode reassembles chunks into the original payload. Out-of-order arrival
// is fine; duplicate chunks tolerated (last one wins). Returns error if any
// chunk fails CRC or chunks are missing.
func Decode(chunks [][]byte) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, errors.New("no chunks")
	}
	var total uint16
	indexed := map[uint16][]byte{}
	for _, c := range chunks {
		if len(c) != ChunkTotalLen {
			return nil, fmt.Errorf("chunk length %d != %d", len(c), ChunkTotalLen)
		}
		crc := crc32.ChecksumIEEE(c[:64])
		if uint16(crc&0xFFFF) != binary.BigEndian.Uint16(c[64:66]) {
			return nil, errors.New("chunk crc mismatch")
		}
		idx := binary.BigEndian.Uint16(c[0:2])
		tot := binary.BigEndian.Uint16(c[2:4])
		if total == 0 {
			total = tot
		} else if total != tot {
			return nil, fmt.Errorf("chunk total mismatch: %d vs %d", total, tot)
		}
		if idx >= total {
			return nil, fmt.Errorf("chunk index %d >= total %d", idx, total)
		}
		indexed[idx] = append([]byte(nil), c[4:64]...)
	}
	if len(indexed) < int(total) {
		return nil, fmt.Errorf("missing chunks: have %d of %d", len(indexed), total)
	}
	out := make([]byte, 0, int(total)*ChunkPayloadLen)
	for i := uint16(0); i < total; i++ {
		chunk, ok := indexed[i]
		if !ok {
			return nil, fmt.Errorf("missing chunk %d", i)
		}
		out = append(out, chunk...)
	}
	return trimPad(out), nil
}

// trimPad removes trailing zero bytes added to fill the last chunk. Caller
// must add a length prefix if their payload legitimately ends in 0x00.
func trimPad(b []byte) []byte {
	i := len(b) - 1
	for i >= 0 && b[i] == 0 {
		i--
	}
	return b[:i+1]
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type encodeResult struct {
	Chunks [][]byte `json:"chunks"`
}

func ApplyEncode(payload string) (encodeResult, error) {
	var in struct {
		Payload []byte `json:"payload"`
	}
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return encodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	chunks, err := Encode(in.Payload)
	if err != nil {
		return encodeResult{}, err
	}
	return encodeResult{Chunks: chunks}, nil
}

type decodeResult struct {
	Payload []byte `json:"payload"`
}

func ApplyDecode(payload string) (decodeResult, error) {
	var in struct {
		Chunks [][]byte `json:"chunks"`
	}
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return decodeResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	out, err := Decode(in.Chunks)
	if err != nil {
		return decodeResult{}, err
	}
	return decodeResult{Payload: out}, nil
}
