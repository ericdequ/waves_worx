// acoustic-rx — listen for BEV sonic discovery frames on stdin PCM
// (intended to be piped from `arecord`), demodulate, then run the real
// sonic.DecodeFrame + sonic.VerifyCohortNonce against the supplied
// lobby secret. Companion to acoustic-tx.
//
// Usage (Terminal 1; start this FIRST so it's listening when TX fires):
//
//	cd GO/mobile
//	arecord -q -f S16_LE -r 48000 -c 1 -t raw 2>/dev/null \
//	  | go run ./bevcore/transport/sonic/cmd/acoustic-rx \
//	      -secret deadbeefcafebabe1122334455667788
//
// The -secret flag must match the TX side for cohort verify to succeed.
// Without it, frames decode structurally but the lobby check is skipped.
//
// If arecord picks the wrong input device, force it: arecord -D plughw:1,0 ...
package main

import (
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
		secretHex = flag.String("secret", "", "expected lobby secret (hex); empty = decode only, no verify")
		tolerance = flag.Int("tol", 3, "cohort verify tolerance in seconds")
		verbose   = flag.Bool("v", false, "log preamble lock events")
	)
	flag.Parse()

	var secret []byte
	if *secretHex != "" {
		var err error
		secret, err = hex.DecodeString(*secretHex)
		if err != nil {
			fatal(fmt.Errorf("-secret: %w", err))
		}
	}

	fmt.Fprintln(os.Stderr, "RX  listening on stdin (raw S16_LE @ 48kHz mono)... Ctrl-C to stop.")
	if secret != nil {
		fmt.Fprintf(os.Stderr, "    secret = %s  (cohort verify ±%ds)\n",
			hex.EncodeToString(secret), *tolerance)
	} else {
		fmt.Fprintln(os.Stderr, "    no -secret provided — cohort verify SKIPPED.")
	}
	fmt.Fprintln(os.Stderr)

	dec := &audiomodem.Decoder{
		NBytes: sonic.FrameLen,
		OnFrame: func(b []byte, snrDb float64) {
			ts := time.Now().Format("15:04:05")
			fmt.Fprintf(os.Stderr, "[%s] rx %d bytes %x  (snr≈%.1fdB)\n", ts, len(b), b, snrDb)
			p, err := sonic.DecodeFrame(b)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [FAIL] sonic.DecodeFrame: %v\n\n", err)
				return
			}
			fmt.Fprintf(os.Stderr, "  kind=%s  bucket=0x%04x  tag=%x  nonce=%x\n",
				kindName(p.Kind), p.TimeBucket, p.PeerTag, p.Nonce)
			if secret != nil {
				if sonic.VerifyCohortNonce(secret, time.Now().Unix(), *tolerance, p.Nonce) {
					fmt.Fprintln(os.Stderr, "  [OK]   cohort nonce verified — same lobby")
				} else {
					fmt.Fprintln(os.Stderr, "  [DROP] cohort nonce rejected — wrong lobby or out-of-tolerance")
				}
			}
			fmt.Fprintln(os.Stderr)
		},
		OnEvent: func(stage, detail string) {
			if *verbose {
				fmt.Fprintf(os.Stderr, "  · %s: %s\n", stage, detail)
			}
		},
	}
	if err := dec.Run(os.Stdin); err != nil {
		fatal(err)
	}
}

func kindName(k byte) string {
	switch k {
	case sonic.KindCohort:
		return "cohort"
	case sonic.KindIdentity:
		return "identity"
	case sonic.KindHandshake:
		return "handshake"
	default:
		return fmt.Sprintf("kind(0x%02x)", k)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "rx:", err)
	os.Exit(1)
}
