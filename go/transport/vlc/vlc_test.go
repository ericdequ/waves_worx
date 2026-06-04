package vlc

import (
	"bytes"
	"testing"
)

func TestRoundtripSmall(t *testing.T) {
	payload := []byte("hello bev")
	chunks, err := Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Errorf("small payload chunks = %d, want 1", len(chunks))
	}
	out, err := Decode(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestRoundtripMultiChunkShuffled(t *testing.T) {
	payload := make([]byte, 250)
	for i := range payload {
		payload[i] = byte(i + 1)
	}
	chunks, err := Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 5 {
		t.Errorf("multi-chunk count = %d, want 5", len(chunks))
	}
	chunks[0], chunks[3] = chunks[3], chunks[0] // shuffle
	out, err := Decode(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestRejectsMissingChunk(t *testing.T) {
	payload := make([]byte, 200)
	for i := range payload {
		payload[i] = byte(i + 1)
	}
	chunks, _ := Encode(payload)
	chunks = chunks[:len(chunks)-1]
	if _, err := Decode(chunks); err == nil {
		t.Errorf("expected error for missing chunk")
	}
}
