package transport

import "sync"

// FallbackChain is an ordered list of vectors to try when the primary
// vector is unavailable. The first entry is conventionally the primary
// vector itself (i.e., "try X first, then fall back to ...").
type FallbackChain []VectorID

type fallbackEntry struct {
	chain    FallbackChain
	platform string // "" = any
}

var (
	fallbackMu       sync.RWMutex
	fallbackRegistry = map[VectorID][]fallbackEntry{}
)

// RegisterFallback registers a fallback chain for a primary vector on a
// specific platform. Call from per-vector packages' init() so platforms
// can register asymmetric chains (NFC is dead on iOS; its iOS chain
// skips NFC entirely).
//
// platform "" means "any platform" and acts as a default when no
// platform-specific chain is registered.
func RegisterFallback(primary VectorID, platform string, chain FallbackChain) {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	fallbackRegistry[primary] = append(fallbackRegistry[primary], fallbackEntry{
		chain:    chain,
		platform: platform,
	})
}

// FallbackOf returns the fallback chain for primary on the given platform.
// Returns the platform-specific chain if registered, else the "any" chain,
// else just [primary] as a degenerate single-element chain.
func FallbackOf(primary VectorID, platform string) FallbackChain {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()
	entries := fallbackRegistry[primary]
	var anyEntry *fallbackEntry
	for i := range entries {
		if entries[i].platform == platform && platform != "" {
			return append(FallbackChain(nil), entries[i].chain...)
		}
		if entries[i].platform == "" && anyEntry == nil {
			anyEntry = &entries[i]
		}
	}
	if anyEntry != nil {
		return append(FallbackChain(nil), anyEntry.chain...)
	}
	return FallbackChain{primary}
}
