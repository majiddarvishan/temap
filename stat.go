package temap

// Stats returns a copy of internal counters.
func (t *TimedMap) Stats() map[string]uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return map[string]uint64{
		"added":     t.stats.added,
		"removed":   t.stats.removed,
		"expired":   t.stats.expired,
		"permanent": t.stats.permanent,
		"current":   uint64(len(t.items)),
	}
}
