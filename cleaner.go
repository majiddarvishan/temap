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
	"time"
)

// --------------------------------------------------------------------
// Cleaner control
// --------------------------------------------------------------------

// StopCleaner gracefully stops background cleaner.
func (t *TimedMap) StopCleaner() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	t.stopped = true
	close(t.stopCh)
	t.mu.Unlock()
	t.wg.Wait()
	t.mu.Lock()
}

// StartCleaner restarts background cleaner if stopped.
func (t *TimedMap) StartCleaner() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startCleaner()
}

// RestartCleaner stops and starts cleaner again.
func (t *TimedMap) RestartCleaner() {
	t.StopCleaner()
	t.StartCleaner()
}

// --------------------------------------------------------------------
// Internal cleaner goroutine
// --------------------------------------------------------------------
func (t *TimedMap) startCleaner() {
	if !t.stopped && t.stopCh != nil {
		return // already running
	}

	t.stopCh = make(chan struct{})
	t.stopped = false
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()

		for {
			t.mu.Lock()
			if len(t.expHeap) == 0 {
				t.mu.Unlock()
				select {
				case <-time.After(time.Second):
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

				for _, el := range expired {
					if t.onExpire != nil {
						go t.onExpire(el.Key, el.Value)
					}
				}
				continue
			}

			t.mu.Unlock()
			select {
			case <-time.After(wait):
				continue
			case <-t.stopCh:
				return
			}
		}
	}()
}
