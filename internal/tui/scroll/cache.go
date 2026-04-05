package scroll

import "sync"

// HeightCache stores rendered message heights (line counts) keyed by message
// index. Heights are cached after first render so the virtual scroll viewport
// does not re-render off-screen messages every frame.
//
// Thread-safe via RWMutex -- reads are concurrent, writes exclusive.
type HeightCache struct {
	heights map[int]int
	mu      sync.RWMutex
}

// NewHeightCache creates an empty height cache.
func NewHeightCache() *HeightCache {
	return &HeightCache{
		heights: make(map[int]int),
	}
}

// Get returns the cached height for the message at idx.
// Returns (height, true) on hit or (0, false) on miss.
func (c *HeightCache) Get(idx int) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.heights[idx]
	return h, ok
}

// Set stores the height for the message at idx.
func (c *HeightCache) Set(idx, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.heights[idx] = height
}

// Invalidate removes the cached height for a single message index.
// Used when a streaming message changes content.
func (c *HeightCache) Invalidate(idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.heights, idx)
}

// InvalidateFrom removes all cached heights for messages at index >= idx.
// Used when the message list is mutated (messages added, removed, or reordered).
func (c *HeightCache) InvalidateFrom(idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.heights {
		if k >= idx {
			delete(c.heights, k)
		}
	}
}

// Clear resets the entire cache.
func (c *HeightCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.heights = make(map[int]int)
}

// Len returns the number of cached entries.
func (c *HeightCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.heights)
}
