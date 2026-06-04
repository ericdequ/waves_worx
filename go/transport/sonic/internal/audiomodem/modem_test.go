package audiomodem

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// TestEncodeDecodeRoundtrip pipes Encode output directly into Decoder
// (zero acoustic channel) and verifies bit-perfect recovery. Failure here
// = modem bug; pass = the demo can only fail due to mic/speaker fidelity.
func TestEncodeDecodeRoundtrip(t *testing.T) {
	in := make([]byte, 16)
	if _, err := rand.Read(in); err != nil {
		t.Fatal(err)
	}
	pcm := Encode(in)

	var got []byte
	dec := &Decoder{
		NBytes: 16,
		OnFrame: func(b []byte, _ float64) {
			got = append([]byte(nil), b...)
		},
	}
	if err := dec.Run(bytes.NewReader(pcm)); err != nil {
		t.Fatalf("decoder run: %v", err)
	}
	if !bytes.Equal(got, in) {
		t.Fatalf("roundtrip mismatch\n  in  = %x\n  got = %x", in, got)
	}
}

// TestEncodeDecodeBackToBack confirms two frames in sequence both decode.
func TestEncodeDecodeBackToBack(t *testing.T) {
	a := bytes.Repeat([]byte{0xA5}, 16)
	b := bytes.Repeat([]byte{0x5A}, 16)

	var buf bytes.Buffer
	buf.Write(Encode(a))
	buf.Write(Encode(b))

	var got [][]byte
	dec := &Decoder{
		NBytes: 16,
		OnFrame: func(b []byte, _ float64) {
			got = append(got, append([]byte(nil), b...))
		},
	}
	if err := dec.Run(&buf); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(got))
	}
	if !bytes.Equal(got[0], a) || !bytes.Equal(got[1], b) {
		t.Fatalf("frames mismatch\n  want=[%x %x]\n  got =[%x %x]", a, b, got[0], got[1])
	}
}
