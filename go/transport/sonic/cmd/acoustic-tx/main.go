// acoustic-tx — emit a BEV sonic discovery frame as audible FSK over
// stdout PCM, intended to be piped to `aplay`. Companion to acoustic-rx.
//
// LAPTOP STAND-IN, not the production transport. Real BEV emits at
// 18-20kHz via the ggwave native plugin; this demo runs at 2-3kHz audible
// because laptop speakers don't reliably reproduce ultrasonic.
//
// Usage (Terminal 2; start the RX side first):
//
//	cd GO/mobile
//	go run ./bevcore/transport/sonic/cmd/acoustic-tx \
//	    -secret deadbeefcafebabe1122334455667788 -kind cohort \
//	  | aplay -q -f S16_LE -r 48000 -c 1
//
// All flags are optional. Defaults: random secret + tag (echoed to
// stderr), single-shot cohort frame.
//
// Loop mode:
//
//	go run ./...acoustic-tx -secret <hex> -loop 5s | aplay ...
//
// Loopback sanity check (no audio hardware):
//
//	go run ./...acoustic-tx -secret <hex> | go run ./...acoustic-rx -secret <hex>
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ericdequ/waves_worx/go/transport/sonic"
	"github.com/ericdequ/waves_worx/go/transport/sonic/internal/audiomodem"
)

func main() {
	var (
		secretHex = flag.String("secret", "", "lobby secret as hex (default: random; printed to stderr)")
		kindArg   = flag.String("kind", "cohort", "frame kind: cohort | handshake | identity")
		tagHex    = flag.String("tag", "", "peer tag as 6-hex-char (default: random)")
		loopEvery = flag.Duration("loop", 0, "repeat every duration (default: single shot)")
	)
	flag.Parse()

	secret, err := decodeOrRandom(*secretHex, 16)
	if err != nil {
		fatal(fmt.Errorf("-secret: %w", err))
	}
	tag, err := decodeOrRandom(*tagHex, 3)
	if err != nil {
		fatal(fmt.Errorf("-tag: %w", err))
	}

	var kind byte
	switch *kindArg {
	case "cohort":
		kind = sonic.KindCohort
	case "handshake":
		kind = sonic.KindHandshake
	case "identity":
		kind = sonic.KindIdentity
	default:
		fatal(fmt.Errorf("unknown -kind %q (want cohort | handshake | identity)", *kindArg))
	}

	fmt.Fprintf(os.Stderr, "TX  secret = %s\n", hex.EncodeToString(secret))
	fmt.Fprintf(os.Stderr, "    tag    = %s\n", hex.EncodeToString(tag))
	fmt.Fprintf(os.Stderr, "    kind   = %s\n", *kindArg)
	fmt.Fprintf(os.Stderr, "    band   = %.0f / %.0f Hz @ %d bps (marker %.0f Hz)\n",
		audiomodem.FreqZero, audiomodem.FreqOne, audiomodem.BitsPerSecond, audiomodem.FreqMarker)

	emit := func() error {
		now := time.Now().Unix()
		frame, err := sonic.EncodeFrame(sonic.Payload{
			Kind:       kind,
			TimeBucket: uint16(now % 65536),
			Nonce:      sonic.CohortNonce(secret, now),
			PeerTag:    tag,
		})
		if err != nil {
			return err
		}
		pcm := audiomodem.Encode(frame)
		durMs := (len(pcm) / 2) * 1000 / audiomodem.SampleRate
		fmt.Fprintf(os.Stderr, "[%s] tx frame=%x  (%dms audio)\n",
			time.Now().Format("15:04:05"), frame, durMs)
		_, err = os.Stdout.Write(pcm)
		return err
	}

	if err := emit(); err != nil {
		fatal(err)
	}
	if *loopEvery > 0 {
		t := time.NewTicker(*loopEvery)
		defer t.Stop()
		for range t.C {
			if err := emit(); err != nil {
				fatal(err)
			}
		}
	}
}

func decodeOrRandom(s string, n int) ([]byte, error) {
	if s == "" {
		b := make([]byte, n)
		_, err := rand.Read(b)
		return b, err
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("expected %d bytes (%d hex chars), got %d", n, n*2, len(b))
	}
	return b, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tx:", err)
	os.Exit(1)
}
