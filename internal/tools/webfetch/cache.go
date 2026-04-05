// Package webfetch URL response cache with TTL and LRU eviction.
package webfetch

import (
	"sort"
	"sync"
	"time"
)

const (
	// CACHE_TTL is the duration after which cached entries expire.
	// Matches the TypeScript implementation's 15-minute cache TTL.
	CACHE_TTL = 15 * time.Minute

	// MAX_CACHE_ENTRIES is the maximum number of entries the cache holds.
	// Once exceeded, the oldest entry is evicted.
	MAX_CACHE_ENTRIES = 100
)

// cacheEntry holds the cached response data for a single URL.
type cacheEntry struct {
	content     string
	contentType string
	statusCode  int
	finalURL    string
	fetchedAt   time.Time
}

// URLCache is a URL-keyed cache with TTL expiration and LRU-style eviction.
type URLCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
}

// NewURLCache creates a new empty URLCache.
func NewURLCache() *URLCache {
	return &URLCache{
		entries: make(map[string]*cacheEntry),
	}
}

// Get returns the cached entry for url if it exists and has not expired.
// Expired entries are deleted on access (lazy eviction).
func (c *URLCache) Get(url string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[url]
	if !ok {
		return nil, false
	}

	// Check TTL expiration
	if time.Since(entry.fetchedAt) > CACHE_TTL {
		delete(c.entries, url)
		return nil, false
	}

	return entry, true
}

// Set adds an entry to the cache. If the cache exceeds MAX_CACHE_ENTRIES,
// the oldest entry (by fetchedAt) is evicted.
func (c *URLCache) Set(url string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[url] = entry

	// Evict oldest entries if over capacity
	for len(c.entries) > MAX_CACHE_ENTRIES {
		c.evictOldest()
	}
}

// Clear removes all entries from the cache.
func (c *URLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Len returns the number of entries in the cache.
func (c *URLCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the entry with the oldest fetchedAt timestamp.
// Must be called with c.mu held.
func (c *URLCache) evictOldest() {
	if len(c.entries) == 0 {
		return
	}

	// Find the entry with the oldest fetchedAt
	type kv struct {
		key       string
		fetchedAt time.Time
	}
	items := make([]kv, 0, len(c.entries))
	for k, v := range c.entries {
		items = append(items, kv{key: k, fetchedAt: v.fetchedAt})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].fetchedAt.Before(items[j].fetchedAt)
	})
	delete(c.entries, items[0].key)
}

// globalCache is the package-level singleton cache used by WebFetchTool.
var globalCache = NewURLCache()
