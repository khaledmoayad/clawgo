// Package filestate implements an LRU file state cache for read-before-edit
// enforcement. It tracks every file read with content, timestamp, offset/limit,
// and a partial-view flag. The Edit and Write tools check this cache to detect
// stale files (content changed since read) and enforce that files must be read
// before being edited. Matches Claude Code's utils/fileStateCache.ts.
package filestate

import (
	"container/list"
	"path/filepath"
	"sync"
)

// ReadFileStateCacheSize is the default maximum number of entries in the cache.
const ReadFileStateCacheSize = 100

// DefaultMaxCacheSizeBytes is the default maximum total content size (25MB).
// This prevents unbounded memory growth from large file contents.
const DefaultMaxCacheSizeBytes = 25 * 1024 * 1024

// FileState tracks the state of a file that has been read.
type FileState struct {
	// Content holds the raw file content as read from disk.
	Content string

	// Timestamp is the Unix timestamp in milliseconds when the file was read.
	Timestamp int64

	// Offset is the 0-indexed line offset of a partial read, or -1 for a full read.
	Offset int

	// Limit is the number of lines read, or -1 for a full read.
	Limit int

	// IsPartialView is true when the cached content was populated by
	// auto-injection (e.g. CLAUDE.md) and the injected content did not match
	// disk (stripped HTML comments, stripped frontmatter, truncated MEMORY.md).
	// The model has only seen a partial view; Edit/Write must require an
	// explicit Read first. Content holds the RAW disk bytes (for diffing),
	// not what the model saw.
	IsPartialView bool
}

// cacheEntry pairs a normalized path key with its file state.
type cacheEntry struct {
	key   string
	state FileState
}

// FileStateCache is a thread-safe LRU cache for file states with both
// entry-count and byte-size eviction limits. All path keys are normalized
// via filepath.Clean before access, ensuring consistent cache hits regardless
// of relative/absolute path variations.
type FileStateCache struct {
	maxEntries   int
	maxSizeBytes int
	currentSize  int
	items        map[string]*list.Element
	order        *list.List
	mu           sync.RWMutex
}

// NewFileStateCache creates a new FileStateCache with the given limits.
func NewFileStateCache(maxEntries int, maxSizeBytes int) *FileStateCache {
	return &FileStateCache{
		maxEntries:   maxEntries,
		maxSizeBytes: maxSizeBytes,
		items:        make(map[string]*list.Element),
		order:        list.New(),
	}
}

// NewDefaultFileStateCache creates a FileStateCache with default limits
// (100 entries, 25MB max content size).
func NewDefaultFileStateCache() *FileStateCache {
	return NewFileStateCache(ReadFileStateCacheSize, DefaultMaxCacheSizeBytes)
}

// entrySize returns the size in bytes of a file state entry.
// Minimum of 1 to ensure every entry consumes at least 1 byte of capacity.
func entrySize(s *FileState) int {
	size := len(s.Content)
	if size < 1 {
		size = 1
	}
	return size
}

// Get retrieves the file state for the given path. The path is normalized
// via filepath.Clean before lookup. On a cache hit, the entry is promoted
// to the front of the LRU list. Returns a copy of the FileState and true
// on hit, or nil and false on miss.
func (c *FileStateCache) Get(key string) (*FileState, bool) {
	key = filepath.Clean(key)

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Promote to front (most recently used)
	c.order.MoveToFront(elem)

	entry := elem.Value.(*cacheEntry)
	// Return a copy so callers cannot mutate internal state
	cp := entry.state
	return &cp, true
}

// Set stores (or updates) the file state for the given path. The path is
// normalized via filepath.Clean. If the cache exceeds maxEntries or
// maxSizeBytes after insertion, the least recently used entries are evicted.
func (c *FileStateCache) Set(key string, state FileState) {
	key = filepath.Clean(key)
	size := entrySize(&state)

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		// Update existing entry in place
		entry := elem.Value.(*cacheEntry)
		oldSize := entrySize(&entry.state)
		entry.state = state
		c.currentSize += size - oldSize
		c.order.MoveToFront(elem)
	} else {
		// Insert new entry at front
		entry := &cacheEntry{key: key, state: state}
		elem := c.order.PushFront(entry)
		c.items[key] = elem
		c.currentSize += size
	}

	// Evict LRU entries until both limits are satisfied
	c.evict()
}

// Has returns true if the cache contains an entry for the given path.
// The path is normalized via filepath.Clean.
func (c *FileStateCache) Has(key string) bool {
	key = filepath.Clean(key)

	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.items[key]
	return ok
}

// Delete removes the entry for the given path. Returns true if the entry
// existed. The path is normalized via filepath.Clean.
func (c *FileStateCache) Delete(key string) bool {
	key = filepath.Clean(key)

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeElement(elem)
	return true
}

// Clear removes all entries from the cache.
func (c *FileStateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
	c.currentSize = 0
}

// Size returns the current number of entries in the cache.
func (c *FileStateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// CurrentSizeBytes returns the current total content size in bytes.
func (c *FileStateCache) CurrentSizeBytes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.currentSize
}

// MaxEntries returns the maximum number of entries allowed.
func (c *FileStateCache) MaxEntries() int {
	return c.maxEntries
}

// MaxSizeBytes returns the maximum total content size in bytes.
func (c *FileStateCache) MaxSizeBytes() int {
	return c.maxSizeBytes
}

// Keys returns all cached paths in LRU order (most recent first).
func (c *FileStateCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for e := c.order.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*cacheEntry)
		keys = append(keys, entry.key)
	}
	return keys
}

// Entries returns a snapshot of all entries as a map from path to FileState.
func (c *FileStateCache) Entries() map[string]FileState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]FileState, len(c.items))
	for e := c.order.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*cacheEntry)
		result[entry.key] = entry.state
	}
	return result
}

// removeElement removes a list element from both the list and the map.
// Caller must hold c.mu write lock.
func (c *FileStateCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	c.currentSize -= entrySize(&entry.state)
	delete(c.items, entry.key)
	c.order.Remove(elem)
}

// evict removes least-recently-used entries until both maxEntries and
// maxSizeBytes limits are satisfied. Caller must hold c.mu write lock.
func (c *FileStateCache) evict() {
	for len(c.items) > c.maxEntries || (c.maxSizeBytes > 0 && c.currentSize > c.maxSizeBytes) {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.removeElement(back)
	}
}

// CloneFileStateCache creates a deep copy of the given cache, preserving
// size limits and all entries in their LRU order.
func CloneFileStateCache(src *FileStateCache) *FileStateCache {
	src.mu.RLock()
	defer src.mu.RUnlock()

	dst := NewFileStateCache(src.maxEntries, src.maxSizeBytes)

	// Walk from back (oldest) to front (newest) so that the newest entries
	// end up at the front of the new cache's LRU list.
	for e := src.order.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*cacheEntry)
		// Copy the state to avoid sharing references
		stateCopy := entry.state
		newEntry := &cacheEntry{key: entry.key, state: stateCopy}
		elem := dst.order.PushFront(newEntry)
		dst.items[entry.key] = elem
		dst.currentSize += entrySize(&stateCopy)
	}

	return dst
}

// MergeFileStateCaches merges two caches. When both contain the same path,
// the entry with the more recent timestamp wins. The result uses the limits
// from the first cache.
func MergeFileStateCaches(first, second *FileStateCache) *FileStateCache {
	merged := CloneFileStateCache(first)

	second.mu.RLock()
	defer second.mu.RUnlock()

	for e := second.order.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*cacheEntry)
		existing, ok := merged.Get(entry.key)
		if !ok || entry.state.Timestamp > existing.Timestamp {
			merged.Set(entry.key, entry.state)
		}
	}

	return merged
}
