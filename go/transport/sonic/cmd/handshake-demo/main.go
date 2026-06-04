// handshake-demo simulates BEV's ultrasonic same-room discovery handshake
// end-to-end against the real sonic package API. No audio is emitted — this
// exercises the frame codec + cohort HMAC schedule + channel allocator the
// native ggwave plugin will sit on top of.
//
// Run from the mobile module:
//
//	cd GO/mobile
//	go run ./bevcore/transport/sonic/cmd/handshake-demo
//
// Three simulated peers:
//
//	Alice   — iPhone 16 (Modern)     | lobby secret S1
//	Bob     — iPhone 13 (Transition) | lobby secret S1
//	Mallory — iPhone 8  (Legacy)     | lobby secret S2  (impostor)
//
// Stages:
//  1. Capability self-report → tier classification → adaptive params
//  2. Channel allocation for the (Alice, Bob) cohort
//  3. Cohort chirp emission + cross-verification
//  4. Clock-skew tolerance (Bob runs +1s ahead)
//  5. Impostor rejection (Mallory's frame verified against S1)
//  6. Handshake frame exchange (Alice → Bob, KindHandshake)
//  7. Frame corruption detection (flip a byte mid-transit)
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/ericdequ/waves_worx/go/transport/sonic"
)

type peer struct {
	id        string
	platform  string
	model     string
	secret    []byte
	clockSkew time.Duration
	tag       []byte // 3-byte ephemeral peer tag
}

func main() {
	failures := 0
	check := func(label string, ok bool) {
		if ok {
			fmt.Printf("  [OK]   %s\n", label)
			return
		}
		fmt.Printf("  [FAIL] %s\n", label)
		failures++
	}
	expectDrop := func(label string, dropped bool) {
		if dropped {
			fmt.Printf("  [DROP] %s\n", label)
			return
		}
		fmt.Printf("  [FAIL] %s (expected drop, got accept)\n", label)
		failures++
	}
	section := func(title string) {
		fmt.Printf("\n── %s ─────────────────────────────────────────\n", title)
	}

	secretS1 := mustRand(32)
	secretS2 := mustRand(32)

	alice := peer{
		id: "alice", platform: "ios", model: "iPhone17,1",
		secret: secretS1, clockSkew: 0,
		tag: mustRand(3),
	}
	bob := peer{
		id: "bob", platform: "ios", model: "iPhone14,5",
		secret: secretS1, clockSkew: 1 * time.Second,
		tag: mustRand(3),
	}
	mallory := peer{
		id: "mallory", platform: "ios", model: "iPhone10,4",
		secret: secretS2, clockSkew: 0,
		tag: mustRand(3),
	}

	fmt.Println("BEV sonic discovery handshake — simulated cohort")
	fmt.Printf("  lobby S1 = %s\n", hex.EncodeToString(secretS1)[:16]+"…")
	fmt.Printf("  lobby S2 = %s (Mallory only)\n", hex.EncodeToString(secretS2)[:16]+"…")

	// ── Stage 1: capability self-report ───────────────────────────────────
	section("Stage 1: capability classification + adaptive params")
	sonic.ClearCapabilities()
	for _, p := range []peer{alice, bob, mallory} {
		tier := sonic.SetCapability(p.id, p.platform, p.model, time.Now().UnixMilli())
		params := sonic.DefaultParams(tier)
		fmt.Printf("  %-8s %-12s → tier=%-10s bitrate=%dbps maxHz=%d vol=%d%% duty=%d%%\n",
			p.id, p.model, tier, params.MaxBitrate, params.CenterFreqCap,
			params.EmitVolumePct, params.ListenDutyPct)
	}
	check("Alice classifies as Modern", sonic.CapabilityOf("alice") == sonic.TierModern)
	check("Bob classifies as Transition", sonic.CapabilityOf("bob") == sonic.TierTransition)
	check("Mallory classifies as Legacy", sonic.CapabilityOf("mallory") == sonic.TierLegacy)

	// ── Stage 2: channel allocation for the Alice+Bob cohort ──────────────
	section("Stage 2: channel allocation (cohort = Alice + Bob)")
	cohortTiers := []sonic.DeviceTier{sonic.CapabilityOf("alice"), sonic.CapabilityOf("bob")}
	ch, degraded := sonic.AllocateChannel(cohortTiers, sonic.PurposeGroupChat)
	fmt.Printf("  groupChat → channel %q  %d–%dHz (center %dHz)  degraded=%v\n",
		ch.ID, ch.LowHz, ch.HighHz, ch.CenterHz, degraded)
	check("groupChat lands on a Transition-tier band", ch.MinTier == sonic.TierTransition)
	check("not degraded to publicVibe fallback", !degraded)

	// What happens if Mallory joins? Lowest tier (Legacy) drops groupChat to public.
	allTiers := append(cohortTiers, sonic.CapabilityOf("mallory"))
	chDeg, degDeg := sonic.AllocateChannel(allTiers, sonic.PurposeGroupChat)
	fmt.Printf("  +Mallory → channel %q                        degraded=%v\n", chDeg.ID, degDeg)
	check("Legacy peer in cohort degrades groupChat → public", degDeg && chDeg.Purpose == sonic.PurposePublicVibe)

	// ── Stage 3: cohort chirp emission + cross-verification ───────────────
	section("Stage 3: cohort chirp (each peer emits, others verify)")
	now := time.Now().Unix()
	aliceFrame := emitCohort(alice, now)
	bobFrame := emitCohort(bob, now+int64(bob.clockSkew.Seconds()))
	fmt.Printf("  Alice emits %d bytes: %s\n", len(aliceFrame), hex.EncodeToString(aliceFrame))
	fmt.Printf("  Bob   emits %d bytes: %s\n", len(bobFrame), hex.EncodeToString(bobFrame))

	// Bob receives Alice's chirp.
	aliceAtBob, err := sonic.DecodeFrame(aliceFrame)
	check("Bob decodes Alice's frame", err == nil && aliceAtBob.Kind == sonic.KindCohort)
	bobAcceptsAlice := sonic.VerifyCohortNonce(alice.secret, now+int64(bob.clockSkew.Seconds()), 2, aliceAtBob.Nonce)
	check("Bob verifies Alice's cohort nonce (same lobby)", bobAcceptsAlice)

	// Alice receives Bob's chirp.
	bobAtAlice, err := sonic.DecodeFrame(bobFrame)
	check("Alice decodes Bob's frame", err == nil)
	aliceAcceptsBob := sonic.VerifyCohortNonce(alice.secret, now, 2, bobAtAlice.Nonce)
	check("Alice verifies Bob's cohort nonce (despite +1s skew)", aliceAcceptsBob)

	// ── Stage 4: clock skew at the edge of tolerance ──────────────────────
	section("Stage 4: clock-skew tolerance (±2s default)")
	for _, delta := range []int64{-2, -1, 0, 1, 2} {
		obs := sonic.CohortNonce(alice.secret, now+delta)
		ok := sonic.VerifyCohortNonce(alice.secret, now, 2, obs)
		check(fmt.Sprintf("Δ=%+ds within tolerance accepted", delta), ok)
	}
	for _, delta := range []int64{-3, 3, 10} {
		obs := sonic.CohortNonce(alice.secret, now+delta)
		expectDrop(fmt.Sprintf("Δ=%+ds outside tolerance rejected", delta),
			!sonic.VerifyCohortNonce(alice.secret, now, 2, obs))
	}

	// ── Stage 5: impostor rejection ───────────────────────────────────────
	section("Stage 5: impostor with wrong lobby secret")
	malloryFrame := emitCohort(mallory, now)
	malloryAtAlice, err := sonic.DecodeFrame(malloryFrame)
	check("Mallory's frame is structurally valid", err == nil)
	malloryAccepted := sonic.VerifyCohortNonce(alice.secret, now, 2, malloryAtAlice.Nonce)
	expectDrop("Alice rejects Mallory's nonce (wrong lobby_secret)", !malloryAccepted)

	// ── Stage 6: handshake exchange (Alice → Bob) ─────────────────────────
	section("Stage 6: handshake frame exchange")
	handshake := sonic.Payload{
		Kind:       sonic.KindHandshake,
		TimeBucket: uint16(now % 65536),
		Nonce:      sonic.CohortNonce(alice.secret, now), // lobby-bound
		PeerTag:    alice.tag,                            // "I am Alice"
	}
	hsFrame, err := sonic.EncodeFrame(handshake)
	check("Alice encodes handshake frame", err == nil && len(hsFrame) == sonic.FrameLen)

	got, err := sonic.DecodeFrame(hsFrame)
	check("Bob decodes handshake frame", err == nil)
	check("Kind = handshake", got.Kind == sonic.KindHandshake)
	check("PeerTag matches Alice's ephemeral tag", bytes.Equal(got.PeerTag, alice.tag))
	check("Handshake nonce verifies against lobby S1",
		sonic.VerifyCohortNonce(alice.secret, now, 2, got.Nonce))

	// ── Stage 7: frame corruption (XOR checksum catches it) ───────────────
	section("Stage 7: frame corruption mid-transit")
	corrupted := append([]byte(nil), aliceFrame...)
	corrupted[6] ^= 0x01 // flip a bit in the nonce region
	_, err = sonic.DecodeFrame(corrupted)
	expectDrop("single-bit flip in nonce caught by checksum", err != nil)

	truncated := aliceFrame[:sonic.FrameLen-1]
	_, err = sonic.DecodeFrame(truncated)
	expectDrop("truncated frame rejected", err != nil)

	// ── Summary ───────────────────────────────────────────────────────────
	section("Summary")
	if failures == 0 {
		fmt.Println("  All discovery-handshake stages passed.")
		fmt.Println("  Next gate: ggwave native plugin (see docs/refactor/ULTRASONIC_VIBE.md Phase 0).")
		os.Exit(0)
	}
	fmt.Printf("  %d failure(s). See [FAIL] lines above.\n", failures)
	os.Exit(1)
}

func emitCohort(p peer, atSec int64) []byte {
	frame, err := sonic.EncodeFrame(sonic.Payload{
		Kind:       sonic.KindCohort,
		TimeBucket: uint16(atSec % 65536),
		Nonce:      sonic.CohortNonce(p.secret, atSec),
		PeerTag:    p.tag,
	})
	if err != nil {
		panic(fmt.Sprintf("%s emit: %v", p.id, err))
	}
	return frame
}

func mustRand(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
