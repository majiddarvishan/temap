/*
 * Copyright (c) 2020 Firas M. Darwish ( https://firas.dev.sy )
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package temap

import (
	"container/heap"
	"sync"
	"time"
)

const (
	ElementDoesntExist = -1
	ElementPermanent   = 0
)

type TimedMap struct {
	mu       sync.RWMutex
	items    map[any]*element
	expHeap  expiryHeap
	onExpire func(key, val any)

	stopCh chan struct{}
	wg     sync.WaitGroup

	stopped bool // indicates if cleaner is currently stopped

	stats struct {
		added     uint64
		removed   uint64
		expired   uint64
		permanent uint64
	}
}

// New creates a TimedMap with a background cleaner.
func New(onExpire func(key, val any)) *TimedMap {
	tm := &TimedMap{
		items:    make(map[any]*element),
		onExpire: onExpire,
		stopCh:   make(chan struct{}),
	}
	heap.Init(&tm.expHeap)
	tm.startCleaner()
	return tm
}

// func New(interval time.Duration, timeout_callback func(key, val any)) *TimedMap {
// 	t := &TimedMap{
// 		tmap:              map[any]*element{},
// 		mu:                &sync.RWMutex{},
// 		cleanerInterval:   interval,
// 		cleanerTicker:     time.NewTicker(interval),
// 		stopCleanerTicker: make(chan bool),
// 		stoppedCleaner:    true,
// 		onTimeout:         timeout_callback,
// 	}

// 	t.StartCleaner()

// 	return t
// }


// SetTemporary sets a key with explicit expiration time.
func (t *TimedMap) SetTemporary(key, value any, expiresAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	exp := expiresAt.UnixNano()
	if el, ok := t.items[key]; ok {
		el.Value = value
		el.ExpiresAt = exp
		if el.ExpiresAt != ElementPermanent {
			heap.Fix(&t.expHeap, el.index)
		}
	} else {
		el := &element{Key: key, Value: value, ExpiresAt: exp}
		t.items[key] = el
		if exp != ElementPermanent {
			heap.Push(&t.expHeap, el)
		} else {
			t.stats.permanent++
		}
		t.stats.added++
	}
}

// SetWithTTL sets a key that expires after the given TTL duration.
func (t *TimedMap) SetWithTTL(key, value any, ttl time.Duration) {
	if ttl <= 0 {
		t.SetPermanent(key, value)
		return
	}
	t.SetTemporary(key, value, time.Now().Add(ttl))
}

// SetPermanent sets a key that never expires.
func (t *TimedMap) SetPermanent(key, value any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if el, ok := t.items[key]; ok {
		el.Value = value
		el.ExpiresAt = ElementPermanent
	} else {
		t.items[key] = &element{Key: key, Value: value, ExpiresAt: ElementPermanent}
		t.stats.added++
		t.stats.permanent++
	}
}

// Get retrieves a value and its expiration.
func (t *TimedMap) Get(key any) (any, int64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	el, ok := t.items[key]
	if !ok {
		return nil, ElementDoesntExist, false
	}
	return el.Value, el.ExpiresAt, true
}

// Remove deletes a key.
func (t *TimedMap) Remove(key any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if el, ok := t.items[key]; ok {
		delete(t.items, key)
		if el.ExpiresAt != ElementPermanent && el.index >= 0 && el.index < len(t.expHeap) {
			heap.Remove(&t.expHeap, el.index)
		}
		t.stats.removed++
	}
}

// RemoveAll clears all entries.
func (t *TimedMap) RemoveAll() {
	t.mu.Lock()
	t.items = make(map[any]*element)
	t.expHeap = expiryHeap{}
	heap.Init(&t.expHeap)
	t.mu.Unlock()
}

// Size returns current number of items.
func (t *TimedMap) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.items)
}

// MakePermanent marks an existing key as permanent (non-expiring).
// Returns true if the key existed and was made permanent, false otherwise.
func (t *TimedMap) MakePermanent(key any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.items[key]
	if !ok || el == nil {
		return false
	}

	// Already permanent
	if el.ExpiresAt == ElementPermanent {
		return true
	}

	// Remove from heap if it was scheduled for expiry
	if el.index >= 0 && el.index < len(t.expHeap) {
		heap.Remove(&t.expHeap, el.index)
	}

	el.ExpiresAt = ElementPermanent
	t.stats.permanent++
	return true
}

// SetExpiry updates the expiry time of an existing key.
// Returns true if the key exists and the expiry was updated successfully, false otherwise.
//
// If expiresAt.IsZero(), the key is made permanent.
// If the key is already expired, it will be removed and false is returned.
func (t *TimedMap) SetExpiry(key any, expiresAt time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.items[key]
	if !ok || el == nil {
		return false
	}

	// Handle permanent conversion
	if expiresAt.IsZero() {
		// Already permanent — nothing to do
		if el.ExpiresAt == ElementPermanent {
			return true
		}
		// Remove from heap if previously expiring
		if el.index >= 0 && el.index < len(t.expHeap) {
			heap.Remove(&t.expHeap, el.index)
		}
		el.ExpiresAt = ElementPermanent
		t.stats.permanent++
		return true
	}

	newExp := expiresAt.UnixNano()
	now := time.Now().UnixNano()

	// If already expired relative to now, remove immediately
	if newExp <= now {
		if el.ExpiresAt != ElementPermanent && el.index >= 0 {
			heap.Remove(&t.expHeap, el.index)
		}
		delete(t.items, key)
		t.stats.removed++
		return false
	}

	// If previously permanent, now becomes expiring — push into heap
	if el.ExpiresAt == ElementPermanent {
		el.ExpiresAt = newExp
		heap.Push(&t.expHeap, el)
		return true
	}

	// If already in heap, adjust its position
	el.ExpiresAt = newExp
	if el.index >= 0 && el.index < len(t.expHeap) {
		heap.Fix(&t.expHeap, el.index)
	}
	return true
}
