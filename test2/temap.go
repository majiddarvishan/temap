package main

import (
	"sync"
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
}

// New creates a new TTLMap with optional expiration callback
func New(callback ExpirationCallback) *TTLMap {
	return &TTLMap{
		items:    make(map[string]*item),
		callback: callback,
	}
}

// SetTemporary adds a key-value pair with TTL
func (m *TTLMap) SetTemporary(key string, value interface{}, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel existing timer if present
	if existing, ok := m.items[key]; ok && existing.timer != nil {
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
}

// SetPermanent adds a key-value pair without expiration
func (m *TTLMap) SetPermanent(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel existing timer if present
	if existing, ok := m.items[key]; ok && existing.timer != nil {
		existing.timer.Stop()
	}

	m.items[key] = &item{
		value:      value,
		expiration: 0,
		timer:      nil,
	}
}

// Get retrieves a value by key, returns nil if not found or expired
func (m *TTLMap) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	itm, ok := m.items[key]
	if !ok {
		return nil, false
	}

	// Check if expired (shouldn't happen due to timer, but extra safety)
	if itm.expiration > 0 && time.Now().UnixNano() > itm.expiration {
		return nil, false
	}

	return itm.value, true
}

// Remove deletes a key and cancels its timer
func (m *TTLMap) Remove(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	itm, ok := m.items[key]
	if !ok {
		return false
	}

	if itm.timer != nil {
		itm.timer.Stop()
	}

	delete(m.items, key)
	return true
}

// RemoveAll clears all entries and cancels all timers
func (m *TTLMap) RemoveAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, itm := range m.items {
		if itm.timer != nil {
			itm.timer.Stop()
		}
	}

	m.items = make(map[string]*item)
}

// Size returns the current number of items in the map
func (m *TTLMap) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
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
	m.mu.Unlock()

	// Call callback outside of lock to avoid deadlocks
	if m.callback != nil {
		m.callback(key, value)
	}
}
