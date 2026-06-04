package sonic

import "testing"

func TestDefaultChannelsHaveGuardSpace(t *testing.T) {
	// Verify ≥200 Hz guard space between consecutive channel bands.
	for i := 1; i < len(DefaultChannels); i += 1 {
		prev := DefaultChannels[i-1]
		curr := DefaultChannels[i]
		gap := curr.LowHz - prev.HighHz
		if gap < 200 {
			t.Errorf("guard space too small between %s and %s: %d Hz", prev.ID, curr.ID, gap)
		}
	}
}

func TestDefaultChannelsAllAboveLegacyAudible(t *testing.T) {
	// Public channel is allowed to dip to 18.5 kHz (young kids may hear);
	// every higher-tier channel must be ≥19 kHz.
	for _, c := range DefaultChannels {
		if c.MinTier == TierLegacy {
			continue
		}
		if c.LowHz < 19000 {
			t.Errorf("non-legacy channel %s starts below 19 kHz (%d Hz)", c.ID, c.LowHz)
		}
	}
}

func TestAllocateChannelByPurpose(t *testing.T) {
	tiers := []DeviceTier{TierModern, TierModern}
	ch, fb := AllocateChannel(tiers, PurposeGroupChat)
	if fb {
		t.Errorf("modern peers shouldn't fall back: got %s", ch.ID)
	}
	if ch.Purpose != PurposeGroupChat {
		t.Errorf("got purpose %s, want groupChat", ch.Purpose)
	}
}

func TestAllocateChannelLegacyDowngradesToPublic(t *testing.T) {
	tiers := []DeviceTier{TierLegacy, TierModern}
	ch, fb := AllocateChannel(tiers, PurposePairing)
	if !fb {
		t.Errorf("legacy+modern asking for pairing should fall back")
	}
	if ch.ID != "public" {
		t.Errorf("fallback should be public, got %s", ch.ID)
	}
}

func TestAllocateChannelSkipsDegraded(t *testing.T) {
	healthMu.Lock()
	health["groupA"] = channelHealth{SNR: 3, PacketLoss: 0.5, UpdatedMs: 1}
	healthMu.Unlock()
	defer func() {
		healthMu.Lock()
		delete(health, "groupA")
		healthMu.Unlock()
	}()

	ch, _ := AllocateChannel([]DeviceTier{TierModern, TierModern}, PurposeGroupChat)
	if ch.ID == "groupA" {
		t.Errorf("allocator returned degraded channel: %s", ch.ID)
	}
	if ch.ID != "groupB" {
		t.Errorf("expected groupB, got %s", ch.ID)
	}
}

func TestChannelByIDMiss(t *testing.T) {
	if _, ok := ChannelByID("nope"); ok {
		t.Errorf("invented a channel that doesn't exist")
	}
}

func TestIsDegradedThresholds(t *testing.T) {
	cases := []struct {
		snr      float64
		loss     float64
		degraded bool
	}{
		{20, 0.0, false},  // healthy
		{9, 0.0, true},    // bad SNR
		{20, 0.5, true},   // bad packet loss
		{15, 0.20, false}, // borderline OK
	}
	for _, c := range cases {
		healthMu.Lock()
		health["public"] = channelHealth{SNR: c.snr, PacketLoss: c.loss, UpdatedMs: 1}
		healthMu.Unlock()
		got := IsDegraded("public")
		if got != c.degraded {
			t.Errorf("snr=%v loss=%v: got degraded=%v, want %v", c.snr, c.loss, got, c.degraded)
		}
	}
	healthMu.Lock()
	delete(health, "public")
	healthMu.Unlock()
}
