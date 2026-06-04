package transport

import "sync"

var (
	globalAggregator     *Aggregator
	globalAggregatorOnce sync.Once
	globalPlatform       string
	globalPlatformMu     sync.RWMutex
)

// GlobalAggregator returns the package-singleton aggregator that the cross-
// vector Apply ops read + write. Per-vector subpackages should not touch this
// directly; they go through Record() / TrustOf() via Apply ops.
func GlobalAggregator() *Aggregator {
	globalAggregatorOnce.Do(func() { globalAggregator = NewAggregator() })
	return globalAggregator
}

// SetPlatform tells the transport layer which platform we're on
// ("ios"/"android"/"web"). Used by FallbackOf to choose the right chain.
// Called once at boot via Apply op transport.set_platform.
func SetPlatform(p string) {
	globalPlatformMu.Lock()
	globalPlatform = p
	globalPlatformMu.Unlock()
}

// Platform returns the currently-set platform string.
func Platform() string {
	globalPlatformMu.RLock()
	defer globalPlatformMu.RUnlock()
	return globalPlatform
}
