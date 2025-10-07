package main

import (
	"container/heap"
	"sync"
	"time"
)

const (
	ElementDoesntExist = -1
	ElementPermanent   = 0
)

// --------------------------------------------------------------------
// Internal element + heap
// --------------------------------------------------------------------
type element struct {
	Key       any
	Value     any
	ExpiresAt int64 // UnixNano timestamp
	index     int
}

type expiryHeap []*element

func (h expiryHeap) Len() int           { return len(h) }
func (h expiryHeap) Less(i, j int) bool { return h[i].ExpiresAt < h[j].ExpiresAt }
func (h expiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *expiryHeap) Push(x any) {
	el := x.(*element)
	el.index = len(*h)
	*h = append(*h, el)
}

func (h *expiryHeap) Pop() any {
	old := *h
	n := len(old)
	el := old[n-1]
	old[n-1] = nil
	el.index = -1
	*h = old[:n-1]
	return el
}

// --------------------------------------------------------------------
// TimedMap
// --------------------------------------------------------------------
type TimedMap struct {
	mu       sync.RWMutex
	items    map[any]*element
	expHeap  expiryHeap
	onExpire func(key, val any)

	stopCh        chan struct{}
	signalCleaner func()
	wg            sync.WaitGroup
	stopped       bool

	// Worker pool for callbacks
	expireWorkers int
	expireQueue   chan *element
	expireWg      sync.WaitGroup

	stats struct {
		added     uint64
		removed   uint64
		expired   uint64
		permanent uint64
	}
}

// --------------------------------------------------------------------
// Constructors
// --------------------------------------------------------------------
func New(onExpire func(key, val any), workerCount int) *TimedMap {
	if workerCount <= 0 {
		workerCount = 4 // default
	}
	tm := &TimedMap{
		items:         make(map[any]*element),
		onExpire:      onExpire,
		expireWorkers: workerCount,
		expireQueue:   make(chan *element, 1024),
	}
	heap.Init(&tm.expHeap)
	tm.startExpireWorkers()
	tm.startCleaner()
	return tm
}

// Start a bounded worker pool for expiration callbacks
func (t *TimedMap) startExpireWorkers() {
	for i := 0; i < t.expireWorkers; i++ {
		t.expireWg.Add(1)
		go func() {
			defer t.expireWg.Done()
			for el := range t.expireQueue {
				if t.onExpire != nil {
					t.onExpire(el.Key, el.Value)
				}
			}
		}()
	}
}

// --------------------------------------------------------------------
// Set methods
// --------------------------------------------------------------------
func (t *TimedMap) SetTemporary(key, value any, expiresAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	exp := expiresAt.UnixNano()
	if el, ok := t.items[key]; ok {
		el.Value = value
		if el.ExpiresAt != ElementPermanent {
			el.ExpiresAt = exp
			if el.index >= 0 {
				heap.Fix(&t.expHeap, el.index)
			} else {
				heap.Push(&t.expHeap, el)
			}
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

	if t.signalCleaner != nil && len(t.expHeap) > 0 && t.expHeap[0] == t.items[key] {
		t.signalCleaner()
	}
}

func (t *TimedMap) SetWithTTL(key, value any, ttl time.Duration) {
	if ttl <= 0 {
		t.SetPermanent(key, value)
		return
	}
	t.SetTemporary(key, value, time.Now().Add(ttl))
}

func (t *TimedMap) SetPermanent(key, value any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if el, ok := t.items[key]; ok {
		el.Value = value
		if el.ExpiresAt != ElementPermanent {
			if el.index >= 0 {
				heap.Remove(&t.expHeap, el.index)
			}
			t.stats.permanent++
		}
		el.ExpiresAt = ElementPermanent
	} else {
		t.items[key] = &element{Key: key, Value: value, ExpiresAt: ElementPermanent}
		t.stats.added++
		t.stats.permanent++
	}
}

func (t *TimedMap) SetExpiry(key any, expiresAt time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.items[key]
	if !ok || el == nil {
		return false
	}

	if expiresAt.IsZero() {
		if el.ExpiresAt != ElementPermanent {
			if el.index >= 0 {
				heap.Remove(&t.expHeap, el.index)
			}
			t.stats.permanent++
		}
		el.ExpiresAt = ElementPermanent
		return true
	}

	newExp := expiresAt.UnixNano()
	now := time.Now().UnixNano()

	if newExp <= now {
		if el.ExpiresAt != ElementPermanent && el.index >= 0 {
			heap.Remove(&t.expHeap, el.index)
		}
		delete(t.items, key)
		t.stats.removed++
		return false
	}

	if el.ExpiresAt == ElementPermanent {
		el.ExpiresAt = newExp
		heap.Push(&t.expHeap, el)
	} else {
		el.ExpiresAt = newExp
		if el.index >= 0 {
			heap.Fix(&t.expHeap, el.index)
		}
	}

	if t.signalCleaner != nil && len(t.expHeap) > 0 && t.expHeap[0] == el {
		t.signalCleaner()
	}

	return true
}

// --------------------------------------------------------------------
// Get / Remove
// --------------------------------------------------------------------
func (t *TimedMap) Get(key any) (any, int64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	el, ok := t.items[key]
	if !ok {
		return nil, ElementDoesntExist, false
	}
	return el.Value, el.ExpiresAt, true
}

func (t *TimedMap) Remove(key any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.items[key]
	if !ok {
		return
	}
	delete(t.items, key)
	if el.ExpiresAt != ElementPermanent && el.index >= 0 {
		heap.Remove(&t.expHeap, el.index)
	}
	t.stats.removed++
}

func (t *TimedMap) RemoveAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.items = make(map[any]*element)
	t.expHeap = expiryHeap{}
	heap.Init(&t.expHeap)
}

// --------------------------------------------------------------------
// MakePermanent
// --------------------------------------------------------------------
func (t *TimedMap) MakePermanent(key any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	el, ok := t.items[key]
	if !ok || el == nil {
		return false
	}
	if el.ExpiresAt != ElementPermanent {
		if el.index >= 0 {
			heap.Remove(&t.expHeap, el.index)
		}
		t.stats.permanent++
	}
	el.ExpiresAt = ElementPermanent
	return true
}

// --------------------------------------------------------------------
// Stats / Snapshot
// --------------------------------------------------------------------
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

func (t *TimedMap) ToMap() map[any]any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[any]any, len(t.items))
	for k, v := range t.items {
		out[k] = v.Value
	}
	return out
}

func (t *TimedMap) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.items)
}

// --------------------------------------------------------------------
// Cleaner
// --------------------------------------------------------------------
func (t *TimedMap) StopCleaner() {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.stopped = true
	close(t.stopCh)
	t.mu.Unlock()
	t.wg.Wait()

	// Stop expire workers
	close(t.expireQueue)
	t.expireWg.Wait()
}

func (t *TimedMap) StartCleaner() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startCleaner()
}

func (t *TimedMap) RestartCleaner() {
	t.StopCleaner()
	t.StartCleaner()
}

// --------------------------------------------------------------------
// Internal cleaner goroutine
// --------------------------------------------------------------------
func (t *TimedMap) startCleaner() {
	if !t.stopped && t.stopCh != nil {
		return
	}

	t.stopCh = make(chan struct{})
	wakeCh := make(chan struct{}, 1)
	t.stopped = false
	t.wg.Add(1)

	t.signalCleaner = func() {
		select {
		case wakeCh <- struct{}{}:
		default:
		}
	}

	go func() {
		defer t.wg.Done()
		var timer *time.Timer

		for {
			t.mu.Lock()
			if len(t.expHeap) == 0 {
				t.mu.Unlock()
				select {
				case <-wakeCh:
					continue
				case <-t.stopCh:
					return
				}
			}

			next := t.expHeap[0]
			wait := time.Until(time.Unix(0, next.ExpiresAt))
			if wait <= 0 {
				expired := []*element{}
				now := time.Now().UnixNano()
				for len(t.expHeap) > 0 && t.expHeap[0].ExpiresAt <= now {
					el := heap.Pop(&t.expHeap).(*element)
					delete(t.items, el.Key)
					expired = append(expired, el)
					t.stats.expired++
				}
				t.mu.Unlock()

				// Send to worker pool
				for _, el := range expired {
					select {
					case t.expireQueue <- el:
					default:
						go func(el *element) { t.onExpire(el.Key, el.Value) }(el)
					}
				}
				continue
			}

			if timer == nil {
				timer = time.NewTimer(wait)
			} else {
				timer.Reset(wait)
			}
			t.mu.Unlock()

			select {
			case <-timer.C:
				continue
			case <-wakeCh:
				if !timer.Stop() {
					<-timer.C
				}
				continue
			case <-t.stopCh:
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}
	}()
}
