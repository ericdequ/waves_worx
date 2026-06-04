package sonic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type allocateInput struct {
	PeerTiers []string `json:"peerTiers"`
	Purpose   string   `json:"purpose"`
}

type allocateResult struct {
	Channel    Channel `json:"channel"`
	Fallback   bool    `json:"fallback"`
	FallbackTo string  `json:"fallbackTo,omitempty"`
}

func ApplyChannelAllocate(payload string) (allocateResult, error) {
	var in allocateInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return allocateResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	tiers := make([]DeviceTier, 0, len(in.PeerTiers))
	for _, t := range in.PeerTiers {
		tiers = append(tiers, DeviceTier(strings.TrimSpace(t)))
	}
	purpose := ChannelPurpose(in.Purpose)
	if purpose == "" {
		purpose = PurposePublicVibe
	}
	ch, fallback := AllocateChannel(tiers, purpose)
	res := allocateResult{Channel: ch, Fallback: fallback}
	if fallback {
		res.FallbackTo = ch.ID
	}
	return res, nil
}

type channelHealthInput struct {
	ChannelID  string  `json:"channelId"`
	SNR        float64 `json:"snr"`
	PacketLoss float64 `json:"packetLoss"`
	UpdatedMs  int64   `json:"updatedMs"`
}

type channelHealthResult struct {
	Recorded bool `json:"recorded"`
	Degraded bool `json:"degraded"`
}

func ApplyChannelHealth(payload string) (channelHealthResult, error) {
	var in channelHealthInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return channelHealthResult{}, fmt.Errorf("invalid payload: %w", err)
	}
	if _, ok := ChannelByID(in.ChannelID); !ok {
		return channelHealthResult{}, errors.New("unknown channel")
	}
	healthMu.Lock()
	health[in.ChannelID] = channelHealth{
		SNR:        in.SNR,
		PacketLoss: in.PacketLoss,
		UpdatedMs:  in.UpdatedMs,
	}
	healthMu.Unlock()
	return channelHealthResult{Recorded: true, Degraded: IsDegraded(in.ChannelID)}, nil
}

type channelsResult struct {
	Channels []Channel `json:"channels"`
}

func ApplyChannels(_ string) (channelsResult, error) {
	out := make([]Channel, len(DefaultChannels))
	copy(out, DefaultChannels)
	return channelsResult{Channels: out}, nil
}
