// Package audiomodem is the laptop-stand-in FSK modem used by the
// acoustic-tx / acoustic-rx demo binaries. It is NOT the production
// transport — the native ggwave plugin (Phase 0 of ULTRASONIC_VIBE.md)
// replaces this on real devices and emits at 18-20kHz instead of the
// audible ~2-3kHz band used here. Internal/ keeps it from being
// imported by anything except the demo cmds.
//
// Wire layout per transmission:
//
//	PreambleMs    of FreqMarker          (sync tone, easy to detect)
//	GapMs         of silence             (gives RX a clean bit-0 anchor)
//	N*8 bits      of FSK data            (MSB-first per byte,
//	                                      bit=0 → FreqZero, bit=1 → FreqOne)
//	PostambleMs   of silence             (lets RX flush + return to idle)
//
// Phase is continuous across tone changes inside Encode to avoid
// audible clicks at frequency boundaries.
package audiomodem

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Modem constants. SamplesPerBit must divide evenly to keep bit
// boundaries on integer sample indices; 48000/25 = 1920 satisfies that.
const (
	SampleRate    = 48000
	BitsPerSecond = 25
	SamplesPerBit = SampleRate / BitsPerSecond // 1920 samples = 40ms per bit

	FreqZero   = 2000.0 // bit = 0
	FreqOne    = 2500.0 // bit = 1
	FreqMarker = 3500.0 // preamble sync tone

	PreambleMs  = 300
	GapMs       = 50
	PostambleMs = 200

	AmpScale = 0.5 // -6dBFS — leave headroom for laptop speakers
)

const samplesPerMs = SampleRate / 1000

// Encode produces raw 16-bit little-endian mono PCM @ 48kHz that, played
// through a speaker, transmits `data` as an FSK chirp. Output is suitable
// for piping directly to `aplay -f S16_LE -r 48000 -c 1`.
func Encode(data []byte) []byte {
	sr := float64(SampleRate)
	capSamples := PreambleMs*samplesPerMs +
		GapMs*samplesPerMs +
		len(data)*8*SamplesPerBit +
		PostambleMs*samplesPerMs
	pcm := make([]int16, 0, capSamples)

	phase := 0.0
	appendTone := func(freq float64, n int) {
		step := 2 * math.Pi * freq / sr
		for i := 0; i < n; i++ {
			pcm = append(pcm, int16(math.Sin(phase)*AmpScale*32767))
			phase += step
			if phase > 2*math.Pi {
				phase -= 2 * math.Pi
			}
		}
	}
	appendSilence := func(n int) {
		for i := 0; i < n; i++ {
			pcm = append(pcm, 0)
		}
		phase = 0
	}

	appendTone(FreqMarker, PreambleMs*samplesPerMs)
	appendSilence(GapMs * samplesPerMs)
	for _, b := range data {
		for bit := 7; bit >= 0; bit-- {
			f := FreqZero
			if (b>>uint(bit))&1 == 1 {
				f = FreqOne
			}
			appendTone(f, SamplesPerBit)
		}
	}
	appendSilence(PostambleMs * samplesPerMs)

	out := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(s))
	}
	return out
}

// Decoder streams 16-bit LE mono PCM from r and decodes each frame of
// NBytes payload into OnFrame. Self-terminates on EOF.
type Decoder struct {
	NBytes  int                               // payload size to expect (e.g. sonic.FrameLen = 16)
	OnFrame func(data []byte, snrDb float64)  // called per decoded frame
	OnEvent func(stage string, detail string) // optional preamble/state notifications
}

const (
	sIdle   = 0
	sInMark = 1
	sInGap  = 2 // marker ended; waiting for the full data tail to arrive
)

// Run reads PCM from r until EOF, calling OnFrame for each decoded payload.
//
// State machine (sliding 10ms windows, 5ms hop):
//
//	idle   → in_mark : marker bin dominates 5 windows in a row
//	in_mark → in_gap : marker bin falls below signal threshold (preamble ended)
//	in_gap → emit    : once the buffer contains the full data tail,
//	                   sample one Goertzel pair per bit slot and pack
func (d *Decoder) Run(r io.Reader) error {
	if d.NBytes <= 0 {
		d.NBytes = 16
	}

	const (
		windowSize = 10 * samplesPerMs //  480 samples
		stepSize   = 5 * samplesPerMs  //  240 samples (5ms hop)
	)

	var samples []float64
	state := sIdle
	consecutiveMarkers := 0
	markerEndIdx := 0
	nextWindow := 0

	raw := make([]byte, 8192)
	for {
		n, err := r.Read(raw)
		if n > 0 {
			for i := 0; i+1 < n; i += 2 {
				u := uint16(raw[i]) | uint16(raw[i+1])<<8
				samples = append(samples, float64(int16(u)))
			}
		}

	inner:
		for {
			if state == sInGap {
				dataStart := markerEndIdx + GapMs*samplesPerMs
				dataEnd := dataStart + d.NBytes*8*SamplesPerBit
				if dataEnd > len(samples) {
					break inner
				}

				bits := make([]int, d.NBytes*8)
				sumSig, sumNoise := 0.0, 0.0
				for i := range bits {
					// Sample the middle 50% of each bit slot to avoid edge artifacts.
					bitStart := dataStart + i*SamplesPerBit + SamplesPerBit/4
					win := samples[bitStart : bitStart+SamplesPerBit/2]
					p0 := goertzel(win, FreqZero)
					p1 := goertzel(win, FreqOne)
					if p1 > p0 {
						bits[i] = 1
						sumSig += p1
						sumNoise += p0
					} else {
						sumSig += p0
						sumNoise += p1
					}
				}

				out := make([]byte, d.NBytes)
				for i, b := range bits {
					if b == 1 {
						out[i/8] |= 1 << uint(7-(i%8))
					}
				}
				snrDb := 10 * math.Log10((sumSig+1)/(sumNoise+1))
				if d.OnFrame != nil {
					d.OnFrame(out, snrDb)
				}

				// Consume the frame + reset; copy tail to fresh array so the
				// underlying slice can be GC'd between frames in -loop mode.
				tail := append([]float64(nil), samples[dataEnd:]...)
				samples = tail
				state = sIdle
				consecutiveMarkers = 0
				markerEndIdx = 0
				nextWindow = 0
				continue
			}

			if nextWindow+windowSize > len(samples) {
				break inner
			}
			w := samples[nextWindow : nextWindow+windowSize]
			pMarker := goertzel(w, FreqMarker)
			p0 := goertzel(w, FreqZero)
			p1 := goertzel(w, FreqOne)
			pSig := p0
			if p1 > pSig {
				pSig = p1
			}
			if pSig < 1.0 {
				pSig = 1.0
			}

			switch state {
			case sIdle:
				if pMarker > 5*pSig {
					consecutiveMarkers++
					if consecutiveMarkers >= 5 {
						state = sInMark
						if d.OnEvent != nil {
							d.OnEvent("preamble", fmt.Sprintf("locked at sample %d", nextWindow))
						}
					}
				} else {
					consecutiveMarkers = 0
				}
			case sInMark:
				if pMarker < 2*pSig {
					markerEndIdx = nextWindow
					state = sInGap
				}
			}
			nextWindow += stepSize
		}

		// Keep idle-state buffer bounded — drop a couple of seconds of
		// old audio once it accumulates with no preamble detection.
		if state == sIdle && nextWindow > 2*SampleRate {
			tail := append([]float64(nil), samples[nextWindow:]...)
			samples = tail
			nextWindow = 0
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// goertzel computes squared magnitude at `freq` over `samples`. O(N), no FFT.
func goertzel(samples []float64, freq float64) float64 {
	N := len(samples)
	k := int(0.5 + float64(N)*freq/float64(SampleRate))
	w := 2 * math.Pi * float64(k) / float64(N)
	coeff := 2 * math.Cos(w)
	var s1, s2 float64
	for _, x := range samples {
		s := x + coeff*s1 - s2
		s2 = s1
		s1 = s
	}
	return s1*s1 + s2*s2 - coeff*s1*s2
}
