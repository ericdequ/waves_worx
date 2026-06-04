package sonic

import "testing"

func TestClassifyIPhoneByIdentifier(t *testing.T) {
	cases := []struct {
		model string
		want  DeviceTier
	}{
		{"iPhone10,3", TierLegacy},     // iPhone X
		{"iPhone10,6", TierLegacy},     // iPhone X
		{"iPhone11,8", TierTransition}, // iPhone XR
		{"iPhone12,1", TierTransition}, // iPhone 11
		{"iPhone14,5", TierTransition}, // iPhone 13
		{"iPhone15,2", TierTransition}, // iPhone 14
		{"iPhone16,1", TierModern},     // iPhone 15
		{"iPhone17,1", TierModern},     // iPhone 16
		{"iPhone20,1", TierModern},     // future
	}
	for _, c := range cases {
		got := ClassifyTier("ios", c.model)
		if got != c.want {
			t.Errorf("model=%s: got %s, want %s", c.model, got, c.want)
		}
	}
}

func TestClassifyIPhoneHumanString(t *testing.T) {
	cases := []struct {
		model string
		want  DeviceTier
	}{
		{"iPhone 8 Plus", TierLegacy},
		{"iPhone 11 Pro Max", TierTransition},
		{"iPhone 15 Pro", TierModern},
	}
	for _, c := range cases {
		got := ClassifyTier("ios", c.model)
		if got != c.want {
			t.Errorf("model=%s: got %s, want %s", c.model, got, c.want)
		}
	}
}

func TestClassifyAndroidPixel(t *testing.T) {
	if got := ClassifyTier("android", "Pixel 8 Pro"); got != TierModern {
		t.Errorf("Pixel 8 Pro: got %s, want modern", got)
	}
	if got := ClassifyTier("android", "Pixel 6"); got != TierTransition {
		t.Errorf("Pixel 6: got %s, want transition", got)
	}
	if got := ClassifyTier("android", "Pixel 4a"); got != TierLegacy {
		t.Errorf("Pixel 4a: got %s, want legacy", got)
	}
}

func TestClassifyUnknownDefaultsLegacy(t *testing.T) {
	if got := ClassifyTier("android", "weird-rare-phone"); got != TierLegacy {
		t.Errorf("unknown android: got %s, want legacy (conservative default)", got)
	}
	if got := ClassifyTier("", ""); got != TierLegacy {
		t.Errorf("empty inputs: got %s, want legacy", got)
	}
}

func TestTierAtLeast(t *testing.T) {
	if !TierAtLeast(TierModern, TierLegacy) {
		t.Errorf("modern should satisfy legacy minimum")
	}
	if TierAtLeast(TierLegacy, TierModern) {
		t.Errorf("legacy should NOT satisfy modern minimum")
	}
	if !TierAtLeast(TierTransition, TierTransition) {
		t.Errorf("transition should satisfy own minimum")
	}
}

func TestDefaultParamsRiseWithTier(t *testing.T) {
	legacy := DefaultParams(TierLegacy)
	transition := DefaultParams(TierTransition)
	modern := DefaultParams(TierModern)

	if !(legacy.MaxBitrate < transition.MaxBitrate && transition.MaxBitrate < modern.MaxBitrate) {
		t.Errorf("max bitrate should rise with tier")
	}
	if !(legacy.CenterFreqCap < transition.CenterFreqCap && transition.CenterFreqCap < modern.CenterFreqCap) {
		t.Errorf("center freq cap should rise with tier")
	}
	// Listen duty cycle should DROP with tier — modern devices can afford
	// shorter listen windows because their hardware decodes faster.
	if !(legacy.ListenDutyPct > transition.ListenDutyPct && transition.ListenDutyPct > modern.ListenDutyPct) {
		t.Errorf("listen duty should drop with tier")
	}
}

func TestDefaultParamsUseGentlePleasantSoundPolicy(t *testing.T) {
	for _, tier := range []DeviceTier{TierLegacy, TierTransition, TierModern} {
		params := DefaultParams(tier)
		if params.SoundProfile != SoundProfileGentle {
			t.Errorf("%s sound profile = %q, want %q", tier, params.SoundProfile, SoundProfileGentle)
		}
		if params.Envelope != EnvelopeSoftFade {
			t.Errorf("%s envelope = %q, want %q", tier, params.Envelope, EnvelopeSoftFade)
		}
		if !params.UserIntentOnly {
			t.Errorf("%s should require user intent before emitting sound", tier)
		}
		if params.EmitVolumePct > 40 {
			t.Errorf("%s emit volume = %d, want <= 40 for calm room-friendly output", tier, params.EmitVolumePct)
		}
		if params.MaxBurstMs > 500 {
			t.Errorf("%s max burst = %dms, want <= 500ms", tier, params.MaxBurstMs)
		}
		if params.MinRestMs < 1000 {
			t.Errorf("%s min rest = %dms, want >= 1000ms", tier, params.MinRestMs)
		}
	}
}

func TestSetCapabilityCachesPeer(t *testing.T) {
	ClearCapabilities()
	tier := SetCapability("peer-a", "ios", "iPhone16,1", 1000)
	if tier != TierModern {
		t.Errorf("peer-a tier = %s, want modern", tier)
	}
	if got := CapabilityOf("peer-a"); got != TierModern {
		t.Errorf("CapabilityOf peer-a = %s, want modern", got)
	}
}

func TestCapabilityOfUnknownDefaultsLegacy(t *testing.T) {
	ClearCapabilities()
	if got := CapabilityOf("never-set"); got != TierLegacy {
		t.Errorf("unknown peer = %s, want legacy", got)
	}
}
