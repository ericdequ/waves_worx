package sonic

// Device capability tier classification + adaptive sonic parameters.
// JS reports (platform, model) once at boot; Go classifies into one of
// three tiers and exposes the right bitrate / volume / duty-cycle for
// that tier.
//
// Source-of-truth principle: a peer's tier is the LOWEST tier across all
// its observed characteristics. Conservative bias keeps us from
// over-driving an old device's speaker.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// DeviceTier groups devices by acoustic + spatial fidelity.
type DeviceTier string

const (
	TierLegacy     DeviceTier = "legacy"     // iPhone 8/X/SE-1, pre-2020 Android
	TierTransition DeviceTier = "transition" // iPhone 11-14, Pixel 5-7, Galaxy S20-S22
	TierModern     DeviceTier = "modern"     // iPhone 15+, Pixel 8+, Galaxy S23+
)

// tierOrder is an internal ordering for "at least X tier" checks.
// Higher number = more capable.
func tierOrder(t DeviceTier) int {
	switch t {
	case TierLegacy:
		return 1
	case TierTransition:
		return 2
	case TierModern:
		return 3
	}
	return 0
}

// TierAtLeast reports whether `have` meets the minimum tier `min`.
func TierAtLeast(have, min DeviceTier) bool {
	return tierOrder(have) >= tierOrder(min)
}

const (
	// SoundProfileGentle is BEV's only default emission profile. Native
	// audio engines may translate this into near-ultrasonic transport tones
	// or audible game cues, but the result must stay soft, brief, and
	// consent-gated.
	SoundProfileGentle = "gentle_pleasant"

	// EnvelopeSoftFade tells native emitters to use fade-in/fade-out ramps
	// and avoid clicks, alarms, sudden attacks, or harsh notification-style
	// transients.
	EnvelopeSoftFade = "soft_fade"
)

// TierParams is the adaptive parameter bundle for a given tier. Native
// plugin reads these to drive the audio engine — emit volume, bitrate,
// listen duty cycle, and clock-skew tolerance for cohort verify.
//
// BEV sound policy: every emitted sound, including game/session sonic
// proofs, should be healing, peaceful, and pleasant. Data transport stays
// brief and low-impact; audible fallbacks should use soft envelopes and
// never surprise the room.
type TierParams struct {
	Tier              DeviceTier `json:"tier"`
	MaxBitrate        int        `json:"maxBitrate"`        // bps
	CenterFreqCap     int        `json:"centerFreqCap"`     // Hz — won't emit above this
	EmitVolumePct     int        `json:"emitVolumePct"`     // 0..100
	ListenDutyPct     int        `json:"listenDutyPct"`     // 0..100 (e.g., 30 = 30ms-on out of every 100ms)
	CohortToleranceMs int        `json:"cohortToleranceMs"` // wall-clock skew tolerance for verify
	SoundProfile      string     `json:"soundProfile"`      // gentle_pleasant: soft, non-alarming, room-friendly
	Envelope          string     `json:"envelope"`          // soft_fade: no sharp attacks/clicks
	MaxBurstMs        int        `json:"maxBurstMs"`        // max continuous emit window
	MinRestMs         int        `json:"minRestMs"`         // required quiet time between bursts
	UserIntentOnly    bool       `json:"userIntentOnly"`    // never emit from bootstrap/background surprise paths
}

func gentleParams(t DeviceTier, maxBitrate, centerFreqCap, emitVolumePct, listenDutyPct, toleranceMs int) TierParams {
	return TierParams{
		Tier:              t,
		MaxBitrate:        maxBitrate,
		CenterFreqCap:     centerFreqCap,
		EmitVolumePct:     emitVolumePct,
		ListenDutyPct:     listenDutyPct,
		CohortToleranceMs: toleranceMs,
		SoundProfile:      SoundProfileGentle,
		Envelope:          EnvelopeSoftFade,
		MaxBurstMs:        420,
		MinRestMs:         1200,
		UserIntentOnly:    true,
	}
}

// DefaultParams returns the recommended params for each tier. Source of
// truth for SONIC_PRIVATE_LANES.md's adaptive-parameter table.
func DefaultParams(t DeviceTier) TierParams {
	switch t {
	case TierLegacy:
		return gentleParams(TierLegacy, 350, 19000, 28, 45, 3000)
	case TierTransition:
		return gentleParams(TierTransition, 900, 20000, 32, 30, 2000)
	case TierModern:
		return gentleParams(TierModern, 1500, 21500, 36, 20, 1000)
	}
	return DefaultParams(TierLegacy) // conservative
}

// ─── Per-peer capability cache ─────────────────────────────────────────────

type capability struct {
	Platform  string
	Model     string
	Tier      DeviceTier
	UpdatedMs int64
}

var (
	capMu   sync.RWMutex
	selfCap capability
	peerCap = map[string]capability{}
)

// SetCapability records a peer's (or self's) hardware info. peerID = ""
// means "self." Idempotent.
func SetCapability(peerID, platform, model string, updatedMs int64) DeviceTier {
	tier := ClassifyTier(platform, model)
	c := capability{Platform: platform, Model: model, Tier: tier, UpdatedMs: updatedMs}
	capMu.Lock()
	defer capMu.Unlock()
	if peerID == "" {
		selfCap = c
	} else {
		peerCap[peerID] = c
	}
	return tier
}

// CapabilityOf returns the cached tier for a peer (or self if peerID="").
// Defaults to TierLegacy when nothing's been reported — conservative.
func CapabilityOf(peerID string) DeviceTier {
	capMu.RLock()
	defer capMu.RUnlock()
	if peerID == "" {
		if selfCap.Tier == "" {
			return TierLegacy
		}
		return selfCap.Tier
	}
	c, ok := peerCap[peerID]
	if !ok {
		return TierLegacy
	}
	return c.Tier
}

// ClearCapabilities drops every cached entry. Hookable for tests + logout.
func ClearCapabilities() {
	capMu.Lock()
	selfCap = capability{}
	peerCap = map[string]capability{}
	capMu.Unlock()
}

// ─── Model classification ──────────────────────────────────────────────────

// ClassifyTier maps (platform, model) to a DeviceTier. Heuristic — Apple
// doesn't expose `hw.machine` to non-MDM apps reliably, so the model
// string can vary. Conservative defaults: unknown → Legacy.
func ClassifyTier(platform, model string) DeviceTier {
	platform = strings.ToLower(strings.TrimSpace(platform))
	model = strings.TrimSpace(model)
	if model == "" {
		return TierLegacy
	}
	switch platform {
	case "ios":
		return classifyIPhone(model)
	case "android":
		return classifyAndroid(model)
	case "web":
		// Web has no native sonic anyway; tier irrelevant. Default
		// Transition for any param queries (sonic disabled in fallback).
		return TierTransition
	}
	return TierLegacy
}

// iPhone identifier format: "iPhone<gen>,<sub>" (e.g., "iPhone16,1").
// Mapping (as of 2026-05):
//
//	iPhone8,*  = iPhone 6s/SE-1                  → Legacy
//	iPhone9,*  = iPhone 7                        → Legacy
//	iPhone10,* = iPhone 8/8+/X                   → Legacy
//	iPhone11,* = iPhone XR/XS                    → Transition
//	iPhone12,* = iPhone 11 / SE-2                → Transition
//	iPhone13,* = iPhone 12                       → Transition
//	iPhone14,* = iPhone 13 / SE-3                → Transition
//	iPhone15,* = iPhone 14                       → Transition
//	iPhone16,* = iPhone 15                       → Modern
//	iPhone17,* = iPhone 16                       → Modern
//	iPhone18+  = future                          → Modern
var iphoneIDRegex = regexp.MustCompile(`(?i)^iPhone(\d+),`)

func classifyIPhone(model string) DeviceTier {
	// Generic Capacitor return value: "iPhone" (no generation). Default to
	// Transition — most active iPhones in 2026 are in this band.
	if strings.EqualFold(model, "iPhone") {
		return TierTransition
	}
	m := iphoneIDRegex.FindStringSubmatch(model)
	if len(m) < 2 {
		// "iPhone XS" style human-readable strings — best-effort token match.
		lower := strings.ToLower(model)
		switch {
		case strings.Contains(lower, "iphone 8"), strings.Contains(lower, "iphone x ") ||
			strings.HasSuffix(lower, "iphone x"), strings.Contains(lower, "iphone 7"),
			strings.Contains(lower, "iphone 6"), strings.Contains(lower, "iphone se (1"):
			return TierLegacy
		case strings.Contains(lower, "iphone 11"), strings.Contains(lower, "iphone 12"),
			strings.Contains(lower, "iphone 13"), strings.Contains(lower, "iphone 14"):
			return TierTransition
		case strings.Contains(lower, "iphone 15"), strings.Contains(lower, "iphone 16"),
			strings.Contains(lower, "iphone 17"):
			return TierModern
		}
		return TierLegacy
	}
	gen, err := strconv.Atoi(m[1])
	if err != nil {
		return TierLegacy
	}
	switch {
	case gen <= 10:
		return TierLegacy
	case gen <= 15:
		return TierTransition
	default:
		return TierModern
	}
}

// Android model strings are wildly inconsistent. We can't pattern-match
// generation reliably across vendors. The honest path is a native
// capability self-test (emit + listen for our own probe at 19 kHz);
// pass = Transition+. Absent that test, default Legacy.
//
// As a bridge, we recognize a small allow-list of known modern model
// codenames. Extend as we collect data.
var modernAndroidModels = map[string]bool{
	"pixel 8": true, "pixel 9": true, "pixel 10": true,
	"sm-s911": true, "sm-s916": true, "sm-s918": true, // Galaxy S23 series
	"sm-s921": true, "sm-s926": true, "sm-s928": true, // Galaxy S24 series
}

var transitionAndroidModels = map[string]bool{
	"pixel 5": true, "pixel 6": true, "pixel 7": true,
	"sm-g99": true, "sm-g99x": true, // Galaxy S21 family prefix
	"sm-g900": true, "sm-g906": true, // S22 family
}

func classifyAndroid(model string) DeviceTier {
	low := strings.ToLower(model)
	for key := range modernAndroidModels {
		if strings.HasPrefix(low, key) {
			return TierModern
		}
	}
	for key := range transitionAndroidModels {
		if strings.HasPrefix(low, key) {
			return TierTransition
		}
	}
	return TierLegacy
}

// ─── Apply op handlers ─────────────────────────────────────────────────────

type setCapabilityInput struct {
	PeerID    string `json:"peerId,omitempty"` // "" = self
	Platform  string `json:"platform"`
	Model     string `json:"model"`
	UpdatedMs int64  `json:"updatedMs,omitempty"`
}

type setCapabilityResult struct {
	Tier   DeviceTier `json:"tier"`
	Params TierParams `json:"params"`
}

func ApplySetCapability(payload string) (setCapabilityResult, error) {
	var in setCapabilityInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return setCapabilityResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	tier := SetCapability(in.PeerID, in.Platform, in.Model, in.UpdatedMs)
	return setCapabilityResult{Tier: tier, Params: DefaultParams(tier)}, nil
}

type capabilityTierInput struct {
	PeerID string `json:"peerId,omitempty"`
}

type capabilityTierResult struct {
	Tier   DeviceTier `json:"tier"`
	Params TierParams `json:"params"`
}

func ApplyCapabilityTier(payload string) (capabilityTierResult, error) {
	var in capabilityTierInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return capabilityTierResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	tier := CapabilityOf(in.PeerID)
	return capabilityTierResult{Tier: tier, Params: DefaultParams(tier)}, nil
}
