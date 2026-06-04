package sonic

import "sync"

type channelHealth struct {
	SNR        float64
	PacketLoss float64
	UpdatedMs  int64
}

var (
	healthMu sync.RWMutex
	health   = map[string]channelHealth{}
)

// IsDegraded returns true if the channel has poor SNR or high packet loss.
func IsDegraded(channelID string) bool {
	healthMu.RLock()
	defer healthMu.RUnlock()
	h, ok := health[channelID]
	if !ok {
		return false
	}
	return h.SNR < 10 || h.PacketLoss > 0.40
}
