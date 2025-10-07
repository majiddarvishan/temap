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

// --------------------------------------------------------------------
// Internal element + heap (efficient expiry tracking)
// --------------------------------------------------------------------
type element struct {
	Key       any   `json:"key"`
	Value     any   `json:"value"`
	ExpiresAt int64 `json:"expires_at"` // UnixNano timestamp
	index     int   `json:"index"`      // heap index
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
	item := x.(*element)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *expiryHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}
