package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// ExpirationCallback is called when a key expires
type ExpirationCallback func(key string, value interface{})

type item struct {
	value      interface{}
	expiration int64 // Unix nano, 0 = permanent
	timer      *time.Timer
}

// TTLMap is a thread-safe map with TTL support
type TTLMap struct {
	mu       sync.RWMutex
	items    map[string]*item
	callback ExpirationCallback
	size     int64 // atomic counter
}

// New creates a new TTLMap with optional expiration callback
func New(callback ExpirationCallback) *TTLMap {
	return &TTLMap{
		items:    make(map[string]*item),
		callback: callback,
	}
}

// NewWithCapacity creates a new TTLMap with pre-allocated capacity
func NewWithCapacity(capacity int, callback ExpirationCallback) *TTLMap {
	return &TTLMap{
		items:    make(map[string]*item, capacity),
		callback: callback,
	}
}

// SetTemporary adds a key-value pair with TTL
func (m *TTLMap) SetTemporary(key string, value interface{}, ttl time.Duration) {
	m.mu.Lock()

	existing, exists := m.items[key]

	// Cancel existing timer if present
	if exists && existing.timer != nil {
		existing.timer.Stop()
	}

	expiration := time.Now().Add(ttl).UnixNano()

	// Create timer for expiration
	timer := time.AfterFunc(ttl, func() {
		m.expire(key)
	})

	m.items[key] = &item{
		value:      value,
		expiration: expiration,
		timer:      timer,
	}

	if !exists {
		atomic.AddInt64(&m.size, 1)
	}

	m.mu.Unlock()
}

// SetPermanent adds a key-value pair without expiration
func (m *TTLMap) SetPermanent(key string, value interface{}) {
	m.mu.Lock()

	existing, exists := m.items[key]

	// Cancel existing timer if present
	if exists && existing.timer != nil {
		existing.timer.Stop()
	}

	m.items[key] = &item{
		value:      value,
		expiration: 0,
		timer:      nil,
	}

	if !exists {
		atomic.AddInt64(&m.size, 1)
	}

	m.mu.Unlock()
}

// Get retrieves a value by key, returns nil if not found
func (m *TTLMap) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	itm, ok := m.items[key]
	m.mu.RUnlock()

	if !ok {
		return nil, false
	}

	return itm.value, true
}

// GetMultiple retrieves multiple values at once (more efficient than multiple Get calls)
func (m *TTLMap) GetMultiple(keys []string) map[string]interface{} {
	result := make(map[string]interface{}, len(keys))

	m.mu.RLock()
	for _, key := range keys {
		if itm, ok := m.items[key]; ok {
			result[key] = itm.value
		}
	}
	m.mu.RUnlock()

	return result
}

// Remove deletes a key and cancels its timer
func (m *TTLMap) Remove(key string) bool {
	m.mu.Lock()

	itm, ok := m.items[key]
	if !ok {
		m.mu.Unlock()
		return false
	}

	if itm.timer != nil {
		itm.timer.Stop()
	}

	delete(m.items, key)
	atomic.AddInt64(&m.size, -1)

	m.mu.Unlock()
	return true
}

// RemoveMultiple removes multiple keys at once (more efficient than multiple Remove calls)
func (m *TTLMap) RemoveMultiple(keys []string) int {
	m.mu.Lock()

	removed := 0
	for _, key := range keys {
		if itm, ok := m.items[key]; ok {
			if itm.timer != nil {
				itm.timer.Stop()
			}
			delete(m.items, key)
			removed++
		}
	}

	atomic.AddInt64(&m.size, -int64(removed))

	m.mu.Unlock()
	return removed
}

// RemoveAll clears all entries and cancels all timers
func (m *TTLMap) RemoveAll() {
	m.mu.Lock()

	for _, itm := range m.items {
		if itm.timer != nil {
			itm.timer.Stop()
		}
	}

	m.items = make(map[string]*item)
	atomic.StoreInt64(&m.size, 0)

	m.mu.Unlock()
}

// Size returns the current number of items (atomic, no lock needed)
func (m *TTLMap) Size() int {
	return int(atomic.LoadInt64(&m.size))
}

// SetExpiry changes the expiration time of an existing key
// Returns false if key doesn't exist
func (m *TTLMap) SetExpiry(key string, expiresAt time.Time) bool {
	m.mu.Lock()

	itm, ok := m.items[key]
	if !ok {
		m.mu.Unlock()
		return false
	}

	// Cancel existing timer
	if itm.timer != nil {
		itm.timer.Stop()
	}

	now := time.Now()

	// If expiration is in the past, expire immediately
	if expiresAt.Before(now) || expiresAt.Equal(now) {
		value := itm.value
		delete(m.items, key)
		atomic.AddInt64(&m.size, -1)
		m.mu.Unlock()

		if m.callback != nil {
			m.callback(key, value)
		}
		return true
	}

	// Calculate new TTL and set new timer
	ttl := expiresAt.Sub(now)
	itm.expiration = expiresAt.UnixNano()
	itm.timer = time.AfterFunc(ttl, func() {
		m.expire(key)
	})

	m.mu.Unlock()
	return true
}

// Keys returns all current keys (snapshot)
func (m *TTLMap) Keys() []string {
	m.mu.RLock()
	keys := make([]string, 0, len(m.items))
	for k := range m.items {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	return keys
}

// ForEach iterates over all items with a callback (holds read lock during iteration)
func (m *TTLMap) ForEach(fn func(key string, value interface{}) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, itm := range m.items {
		if !fn(k, itm.value) {
			break
		}
	}
}

// expire handles key expiration (called by timer)
func (m *TTLMap) expire(key string) {
	m.mu.Lock()
	itm, ok := m.items[key]
	if !ok {
		m.mu.Unlock()
		return
	}

	value := itm.value
	delete(m.items, key)
	atomic.AddInt64(&m.size, -1)
	m.mu.Unlock()

	// Call callback outside of lock to avoid deadlocks
	if m.callback != nil {
		m.callback(key, value)
	}
}
