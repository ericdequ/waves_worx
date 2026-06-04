package sonic

// ChannelPurpose tags a channel with its intended use case.
type ChannelPurpose string

const (
	PurposePublicVibe ChannelPurpose = "publicVibe"
	PurposeGroupChat  ChannelPurpose = "groupChat"
	PurposePairing    ChannelPurpose = "pairing"
)

// Channel is a named ultrasonic band with a usage policy.
type Channel struct {
	ID       string         `json:"id"`
	LowHz    int            `json:"lowHz"`
	HighHz   int            `json:"highHz"`
	CenterHz int            `json:"centerHz"`
	MinTier  DeviceTier     `json:"minTier"`
	Purpose  ChannelPurpose `json:"purpose"`
}

// Width returns the channel's bandwidth in Hz.
func (c Channel) Width() int { return c.HighHz - c.LowHz }

// DefaultChannels is BEV's canonical channel allocation. Order matters.
var DefaultChannels = []Channel{
	{ID: "public", LowHz: 18500, HighHz: 19000, CenterHz: 18750, MinTier: TierLegacy, Purpose: PurposePublicVibe},
	{ID: "groupA", LowHz: 19200, HighHz: 19500, CenterHz: 19350, MinTier: TierTransition, Purpose: PurposeGroupChat},
	{ID: "groupB", LowHz: 19700, HighHz: 20000, CenterHz: 19850, MinTier: TierTransition, Purpose: PurposeGroupChat},
	{ID: "pairing", LowHz: 21000, HighHz: 21500, CenterHz: 21250, MinTier: TierModern, Purpose: PurposePairing},
}

// ChannelByID looks up a channel by its string ID.
func ChannelByID(id string) (Channel, bool) {
	for _, c := range DefaultChannels {
		if c.ID == id {
			return c, true
		}
	}
	return Channel{}, false
}

// AllocateChannel picks the right channel for the given peer tiers and purpose.
func AllocateChannel(peerTiers []DeviceTier, purpose ChannelPurpose) (Channel, bool) {
	low := minTier(peerTiers)
	for _, c := range DefaultChannels {
		if c.Purpose != purpose {
			continue
		}
		if !TierAtLeast(low, c.MinTier) {
			continue
		}
		if IsDegraded(c.ID) {
			continue
		}
		return c, false
	}
	for _, c := range DefaultChannels {
		if c.Purpose == PurposePublicVibe && !IsDegraded(c.ID) {
			return c, true
		}
	}
	for _, c := range DefaultChannels {
		if c.Purpose == PurposePublicVibe {
			return c, true
		}
	}
	return Channel{}, true
}

func minTier(tiers []DeviceTier) DeviceTier {
	if len(tiers) == 0 {
		return TierLegacy
	}
	m := tiers[0]
	for _, t := range tiers[1:] {
		if tierOrder(t) < tierOrder(m) {
			m = t
		}
	}
	return m
}
