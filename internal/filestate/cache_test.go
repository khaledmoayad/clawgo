package filestate

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestNewDefaultFileStateCache(t *testing.T) {
	c := NewDefaultFileStateCache()
	if c.MaxEntries() != ReadFileStateCacheSize {
		t.Errorf("expected maxEntries=%d, got %d", ReadFileStateCacheSize, c.MaxEntries())
	}
	if c.MaxSizeBytes() != DefaultMaxCacheSizeBytes {
		t.Errorf("expected maxSizeBytes=%d, got %d", DefaultMaxCacheSizeBytes, c.MaxSizeBytes())
	}
	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size=%d", c.Size())
	}
}

func TestGetSetBasic(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/tmp/test.go", FileState{
		Content:   "package main",
		Timestamp: 1000,
		Offset:    -1,
		Limit:     -1,
	})

	got, ok := c.Get("/tmp/test.go")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Content != "package main" {
		t.Errorf("expected content=%q, got %q", "package main", got.Content)
	}
	if got.Timestamp != 1000 {
		t.Errorf("expected timestamp=1000, got %d", got.Timestamp)
	}
	if got.Offset != -1 {
		t.Errorf("expected offset=-1, got %d", got.Offset)
	}
	if got.Limit != -1 {
		t.Errorf("expected limit=-1, got %d", got.Limit)
	}
}

func TestGetReturnsCopy(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/a.go", FileState{Content: "original", Timestamp: 100})

	got, ok := c.Get("/tmp/a.go")
	if !ok {
		t.Fatal("expected cache hit")
	}

	// Mutate the returned copy
	got.Content = "mutated"

	// Verify internal state is unchanged
	got2, _ := c.Get("/tmp/a.go")
	if got2.Content != "original" {
		t.Errorf("internal state was mutated: got %q", got2.Content)
	}
}

func TestGetMiss(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)
	_, ok := c.Get("/tmp/nonexistent.go")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestPathNormalization(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	// Set with a non-normalized path
	c.Set("/tmp/foo/../bar/baz.go", FileState{
		Content:   "normalized",
		Timestamp: 200,
	})

	// Get with the normalized version
	got, ok := c.Get("/tmp/bar/baz.go")
	if !ok {
		t.Fatal("expected cache hit with normalized path")
	}
	if got.Content != "normalized" {
		t.Errorf("expected content=%q, got %q", "normalized", got.Content)
	}

	// Get with a different non-normalized version
	got2, ok := c.Get("/tmp/bar/./baz.go")
	if !ok {
		t.Fatal("expected cache hit with dot-normalized path")
	}
	if got2.Content != "normalized" {
		t.Errorf("expected content=%q, got %q", "normalized", got2.Content)
	}

	// Has should also normalize
	if !c.Has("/tmp/foo/../bar/baz.go") {
		t.Error("Has should find entry with non-normalized path")
	}
	if !c.Has("/tmp/bar/baz.go") {
		t.Error("Has should find entry with normalized path")
	}
}

func TestLRUEvictionByEntries(t *testing.T) {
	// Cache with max 3 entries and large byte limit
	c := NewFileStateCache(3, 100*1024*1024)

	c.Set("/a", FileState{Content: "a", Timestamp: 1})
	c.Set("/b", FileState{Content: "b", Timestamp: 2})
	c.Set("/c", FileState{Content: "c", Timestamp: 3})

	if c.Size() != 3 {
		t.Fatalf("expected size=3, got %d", c.Size())
	}

	// Adding a 4th entry should evict the oldest (/a)
	c.Set("/d", FileState{Content: "d", Timestamp: 4})

	if c.Size() != 3 {
		t.Errorf("expected size=3 after eviction, got %d", c.Size())
	}

	if c.Has("/a") {
		t.Error("/a should have been evicted (oldest)")
	}
	if !c.Has("/b") {
		t.Error("/b should still be in cache")
	}
	if !c.Has("/c") {
		t.Error("/c should still be in cache")
	}
	if !c.Has("/d") {
		t.Error("/d should be in cache")
	}
}

func TestLRUEvictionPromotesOnGet(t *testing.T) {
	c := NewFileStateCache(3, 100*1024*1024)

	c.Set("/a", FileState{Content: "a", Timestamp: 1})
	c.Set("/b", FileState{Content: "b", Timestamp: 2})
	c.Set("/c", FileState{Content: "c", Timestamp: 3})

	// Access /a to promote it to front
	c.Get("/a")

	// Now adding /d should evict /b (the LRU item), not /a
	c.Set("/d", FileState{Content: "d", Timestamp: 4})

	if !c.Has("/a") {
		t.Error("/a should still be in cache (was promoted by Get)")
	}
	if c.Has("/b") {
		t.Error("/b should have been evicted (LRU after /a promotion)")
	}
	if !c.Has("/c") {
		t.Error("/c should still be in cache")
	}
	if !c.Has("/d") {
		t.Error("/d should be in cache")
	}
}

func TestSizeBasedEviction(t *testing.T) {
	// Max 10 bytes total content size, unlimited entries
	c := NewFileStateCache(100, 10)

	c.Set("/a", FileState{Content: "aaaa", Timestamp: 1}) // 4 bytes
	c.Set("/b", FileState{Content: "bbbb", Timestamp: 2}) // 4 bytes = 8 total

	if c.Size() != 2 {
		t.Fatalf("expected size=2, got %d", c.Size())
	}
	if c.CurrentSizeBytes() != 8 {
		t.Fatalf("expected currentSize=8, got %d", c.CurrentSizeBytes())
	}

	// Adding 5 more bytes should push over the 10-byte limit, evicting /a (4 bytes)
	c.Set("/c", FileState{Content: "ccccc", Timestamp: 3}) // 5 bytes = 13 total -> evict /a -> 9

	if c.Has("/a") {
		t.Error("/a should have been evicted due to size limit")
	}
	if !c.Has("/b") {
		t.Error("/b should still be in cache")
	}
	if !c.Has("/c") {
		t.Error("/c should be in cache")
	}
	if c.CurrentSizeBytes() != 9 {
		t.Errorf("expected currentSize=9 after eviction, got %d", c.CurrentSizeBytes())
	}
}

func TestSizeBasedEvictionMultiple(t *testing.T) {
	// Max 8 bytes
	c := NewFileStateCache(100, 8)

	c.Set("/a", FileState{Content: "a", Timestamp: 1})    // 1 byte
	c.Set("/b", FileState{Content: "bb", Timestamp: 2})    // 2 bytes = 3
	c.Set("/c", FileState{Content: "ccc", Timestamp: 3})   // 3 bytes = 6

	// Adding 5 bytes pushes total to 11, max is 8.
	// Evict LRU: /a (1 byte) -> 10. Still over.
	// Evict LRU: /b (2 bytes) -> 8. Now at limit.
	c.Set("/d", FileState{Content: "ddddd", Timestamp: 4}) // 5 bytes

	if c.Has("/a") {
		t.Error("/a should have been evicted")
	}
	if c.Has("/b") {
		t.Error("/b should have been evicted")
	}
	if !c.Has("/c") {
		t.Error("/c should still be in cache")
	}
	if !c.Has("/d") {
		t.Error("/d should be in cache")
	}
	if c.CurrentSizeBytes() != 8 {
		t.Errorf("expected currentSize=8, got %d", c.CurrentSizeBytes())
	}
}

func TestEmptyContentSizeMinimumOne(t *testing.T) {
	c := NewFileStateCache(10, 1024)

	// Empty content should count as 1 byte
	c.Set("/empty", FileState{Content: "", Timestamp: 1})
	if c.CurrentSizeBytes() != 1 {
		t.Errorf("expected currentSize=1 for empty content, got %d", c.CurrentSizeBytes())
	}
}

func TestHasOperation(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	if c.Has("/tmp/test.go") {
		t.Error("Has should return false for nonexistent key")
	}

	c.Set("/tmp/test.go", FileState{Content: "test", Timestamp: 100})

	if !c.Has("/tmp/test.go") {
		t.Error("Has should return true for existing key")
	}
}

func TestDeleteOperation(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/tmp/test.go", FileState{Content: "test", Timestamp: 100})

	if !c.Delete("/tmp/test.go") {
		t.Error("Delete should return true when entry exists")
	}

	if c.Has("/tmp/test.go") {
		t.Error("entry should be gone after Delete")
	}

	if c.Size() != 0 {
		t.Errorf("expected size=0 after delete, got %d", c.Size())
	}

	// Deleting nonexistent entry returns false
	if c.Delete("/tmp/nonexistent.go") {
		t.Error("Delete should return false for nonexistent key")
	}
}

func TestDeleteNormalizes(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/tmp/bar/baz.go", FileState{Content: "test", Timestamp: 100})

	// Delete with non-normalized path
	if !c.Delete("/tmp/foo/../bar/baz.go") {
		t.Error("Delete should normalize path and find entry")
	}

	if c.Has("/tmp/bar/baz.go") {
		t.Error("entry should be gone after normalized delete")
	}
}

func TestClear(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/a", FileState{Content: "a", Timestamp: 1})
	c.Set("/b", FileState{Content: "b", Timestamp: 2})
	c.Set("/c", FileState{Content: "c", Timestamp: 3})

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size=0 after Clear, got %d", c.Size())
	}
	if c.CurrentSizeBytes() != 0 {
		t.Errorf("expected currentSize=0 after Clear, got %d", c.CurrentSizeBytes())
	}
	if c.Has("/a") || c.Has("/b") || c.Has("/c") {
		t.Error("all entries should be gone after Clear")
	}
}

func TestKeys(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/a", FileState{Content: "a", Timestamp: 1})
	c.Set("/b", FileState{Content: "b", Timestamp: 2})
	c.Set("/c", FileState{Content: "c", Timestamp: 3})

	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	// Keys should be in LRU order (most recent first)
	if keys[0] != "/c" || keys[1] != "/b" || keys[2] != "/a" {
		t.Errorf("expected keys in MRU order [/c, /b, /a], got %v", keys)
	}
}

func TestEntries(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/a", FileState{Content: "aaa", Timestamp: 1})
	c.Set("/b", FileState{Content: "bbb", Timestamp: 2})

	entries := c.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries["/a"].Content != "aaa" {
		t.Errorf("expected entry /a content=%q, got %q", "aaa", entries["/a"].Content)
	}
	if entries["/b"].Content != "bbb" {
		t.Errorf("expected entry /b content=%q, got %q", "bbb", entries["/b"].Content)
	}
}

func TestIsPartialViewPreserved(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/claudemd", FileState{
		Content:       "# Claude Config\nSome content",
		Timestamp:     500,
		Offset:        -1,
		Limit:         -1,
		IsPartialView: true,
	})

	got, ok := c.Get("/claudemd")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !got.IsPartialView {
		t.Error("IsPartialView should be true")
	}

	// Also set a non-partial entry
	c.Set("/full", FileState{
		Content:       "full file",
		Timestamp:     600,
		Offset:        -1,
		Limit:         -1,
		IsPartialView: false,
	})

	full, _ := c.Get("/full")
	if full.IsPartialView {
		t.Error("IsPartialView should be false for non-partial entry")
	}
}

func TestOffsetLimitTracked(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	// Full read
	c.Set("/full.go", FileState{
		Content:   "package main\nfunc main() {}",
		Timestamp: 100,
		Offset:    -1,
		Limit:     -1,
	})

	full, _ := c.Get("/full.go")
	if full.Offset != -1 || full.Limit != -1 {
		t.Errorf("full read: expected offset=-1, limit=-1, got offset=%d, limit=%d", full.Offset, full.Limit)
	}

	// Partial read
	c.Set("/partial.go", FileState{
		Content:   "line 10\nline 11\nline 12",
		Timestamp: 200,
		Offset:    10,
		Limit:     3,
	})

	partial, _ := c.Get("/partial.go")
	if partial.Offset != 10 || partial.Limit != 3 {
		t.Errorf("partial read: expected offset=10, limit=3, got offset=%d, limit=%d", partial.Offset, partial.Limit)
	}
}

func TestSetUpdateExisting(t *testing.T) {
	c := NewFileStateCache(10, 1024*1024)

	c.Set("/a", FileState{Content: "old", Timestamp: 100})
	c.Set("/a", FileState{Content: "new content", Timestamp: 200})

	if c.Size() != 1 {
		t.Errorf("expected size=1 after update, got %d", c.Size())
	}

	got, _ := c.Get("/a")
	if got.Content != "new content" {
		t.Errorf("expected updated content=%q, got %q", "new content", got.Content)
	}
	if got.Timestamp != 200 {
		t.Errorf("expected updated timestamp=200, got %d", got.Timestamp)
	}
}

func TestUpdateAdjustsCurrentSize(t *testing.T) {
	c := NewFileStateCache(10, 1024)

	c.Set("/a", FileState{Content: "aaa", Timestamp: 1}) // 3 bytes
	if c.CurrentSizeBytes() != 3 {
		t.Fatalf("expected 3, got %d", c.CurrentSizeBytes())
	}

	c.Set("/a", FileState{Content: "aaaaaa", Timestamp: 2}) // 6 bytes
	if c.CurrentSizeBytes() != 6 {
		t.Errorf("expected 6 after update, got %d", c.CurrentSizeBytes())
	}

	c.Set("/a", FileState{Content: "a", Timestamp: 3}) // 1 byte
	if c.CurrentSizeBytes() != 1 {
		t.Errorf("expected 1 after shrink, got %d", c.CurrentSizeBytes())
	}
}

func TestCloneFileStateCache(t *testing.T) {
	src := NewFileStateCache(10, 1024*1024)
	src.Set("/a", FileState{Content: "aaa", Timestamp: 1})
	src.Set("/b", FileState{Content: "bbb", Timestamp: 2})
	src.Set("/c", FileState{Content: "ccc", Timestamp: 3, IsPartialView: true})

	dst := CloneFileStateCache(src)

	// Check same size
	if dst.Size() != src.Size() {
		t.Errorf("clone size mismatch: src=%d, dst=%d", src.Size(), dst.Size())
	}

	// Check same limits
	if dst.MaxEntries() != src.MaxEntries() {
		t.Errorf("clone maxEntries mismatch: src=%d, dst=%d", src.MaxEntries(), dst.MaxEntries())
	}
	if dst.MaxSizeBytes() != src.MaxSizeBytes() {
		t.Errorf("clone maxSizeBytes mismatch: src=%d, dst=%d", src.MaxSizeBytes(), dst.MaxSizeBytes())
	}

	// Check entries preserved
	for _, key := range []string{"/a", "/b", "/c"} {
		srcState, _ := src.Get(key)
		dstState, ok := dst.Get(key)
		if !ok {
			t.Errorf("clone missing key %s", key)
			continue
		}
		if dstState.Content != srcState.Content {
			t.Errorf("clone content mismatch for %s: src=%q, dst=%q", key, srcState.Content, dstState.Content)
		}
		if dstState.Timestamp != srcState.Timestamp {
			t.Errorf("clone timestamp mismatch for %s: src=%d, dst=%d", key, srcState.Timestamp, dstState.Timestamp)
		}
		if dstState.IsPartialView != srcState.IsPartialView {
			t.Errorf("clone IsPartialView mismatch for %s", key)
		}
	}

	// Verify independence: mutating clone doesn't affect source
	dst.Set("/a", FileState{Content: "modified", Timestamp: 999})
	srcA, _ := src.Get("/a")
	if srcA.Content != "aaa" {
		t.Error("mutating clone affected source")
	}

	// Verify independence: mutating source doesn't affect clone
	src.Delete("/b")
	if !dst.Has("/b") {
		t.Error("deleting from source affected clone")
	}
}

func TestClonePreservesLRUOrder(t *testing.T) {
	src := NewFileStateCache(10, 1024*1024)
	src.Set("/a", FileState{Content: "a", Timestamp: 1})
	src.Set("/b", FileState{Content: "b", Timestamp: 2})
	src.Set("/c", FileState{Content: "c", Timestamp: 3})

	// Promote /a to front
	src.Get("/a")

	dst := CloneFileStateCache(src)
	srcKeys := src.Keys()
	dstKeys := dst.Keys()

	if len(srcKeys) != len(dstKeys) {
		t.Fatalf("key count mismatch: src=%d, dst=%d", len(srcKeys), len(dstKeys))
	}

	for i := range srcKeys {
		if srcKeys[i] != dstKeys[i] {
			t.Errorf("LRU order mismatch at %d: src=%s, dst=%s", i, srcKeys[i], dstKeys[i])
		}
	}
}

func TestMergeFileStateCaches(t *testing.T) {
	first := NewFileStateCache(10, 1024*1024)
	first.Set("/a", FileState{Content: "first-a", Timestamp: 100})
	first.Set("/b", FileState{Content: "first-b", Timestamp: 200})

	second := NewFileStateCache(10, 1024*1024)
	second.Set("/b", FileState{Content: "second-b", Timestamp: 300}) // newer, should win
	second.Set("/c", FileState{Content: "second-c", Timestamp: 400})

	merged := MergeFileStateCaches(first, second)

	// Check /a: only in first
	a, ok := merged.Get("/a")
	if !ok {
		t.Fatal("merged should contain /a from first")
	}
	if a.Content != "first-a" {
		t.Errorf("expected /a content=%q, got %q", "first-a", a.Content)
	}

	// Check /b: second wins (newer timestamp)
	b, ok := merged.Get("/b")
	if !ok {
		t.Fatal("merged should contain /b")
	}
	if b.Content != "second-b" {
		t.Errorf("expected /b content=%q (newer), got %q", "second-b", b.Content)
	}

	// Check /c: only in second
	cc, ok := merged.Get("/c")
	if !ok {
		t.Fatal("merged should contain /c from second")
	}
	if cc.Content != "second-c" {
		t.Errorf("expected /c content=%q, got %q", "second-c", cc.Content)
	}
}

func TestMergeOlderDoesNotOverwrite(t *testing.T) {
	first := NewFileStateCache(10, 1024*1024)
	first.Set("/a", FileState{Content: "first-a", Timestamp: 500})

	second := NewFileStateCache(10, 1024*1024)
	second.Set("/a", FileState{Content: "second-a", Timestamp: 100}) // older, should NOT win

	merged := MergeFileStateCaches(first, second)

	a, _ := merged.Get("/a")
	if a.Content != "first-a" {
		t.Errorf("older entry should not overwrite: expected %q, got %q", "first-a", a.Content)
	}
}

func TestMergeDoesNotMutateInputs(t *testing.T) {
	first := NewFileStateCache(10, 1024*1024)
	first.Set("/a", FileState{Content: "a", Timestamp: 1})

	second := NewFileStateCache(10, 1024*1024)
	second.Set("/b", FileState{Content: "b", Timestamp: 2})

	merged := MergeFileStateCaches(first, second)

	// Mutate merged
	merged.Delete("/a")
	merged.Delete("/b")

	// Verify inputs untouched
	if !first.Has("/a") {
		t.Error("first was mutated by merge")
	}
	if !second.Has("/b") {
		t.Error("second was mutated by merge")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := NewFileStateCache(100, 10*1024*1024)
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("/file-%d.go", n)
			content := strings.Repeat("x", n+1)
			c.Set(key, FileState{
				Content:   content,
				Timestamp: int64(n),
				Offset:    -1,
				Limit:     -1,
			})
		}(i)
	}

	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("/file-%d.go", n)
			c.Get(key)
			c.Has(key)
			c.Size()
			c.Keys()
			c.Entries()
		}(i)
	}

	// Deleters
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("/file-%d.go", n)
			c.Delete(key)
		}(i)
	}

	wg.Wait()

	// Just verify cache is in a consistent state
	size := c.Size()
	keys := c.Keys()
	if len(keys) != size {
		t.Errorf("inconsistent state: Size()=%d, len(Keys())=%d", size, len(keys))
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	c := NewFileStateCache(50, 1024*1024)
	var wg sync.WaitGroup

	// Hammer with concurrent read/write on the SAME key
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			c.Set("/shared", FileState{
				Content:   fmt.Sprintf("content-%d", n),
				Timestamp: int64(n),
			})
		}(i)
		go func(n int) {
			defer wg.Done()
			c.Get("/shared")
		}(i)
	}

	wg.Wait()

	// Should have exactly 1 entry
	if c.Size() != 1 {
		t.Errorf("expected size=1 after concurrent updates to same key, got %d", c.Size())
	}
}

func TestConstants(t *testing.T) {
	if ReadFileStateCacheSize != 100 {
		t.Errorf("ReadFileStateCacheSize should be 100, got %d", ReadFileStateCacheSize)
	}
	if DefaultMaxCacheSizeBytes != 25*1024*1024 {
		t.Errorf("DefaultMaxCacheSizeBytes should be %d, got %d", 25*1024*1024, DefaultMaxCacheSizeBytes)
	}
}

func TestDeleteAdjustsCurrentSize(t *testing.T) {
	c := NewFileStateCache(10, 1024)

	c.Set("/a", FileState{Content: "aaa", Timestamp: 1}) // 3 bytes
	c.Set("/b", FileState{Content: "bb", Timestamp: 2})   // 2 bytes = 5 total

	c.Delete("/a")

	if c.CurrentSizeBytes() != 2 {
		t.Errorf("expected currentSize=2 after delete, got %d", c.CurrentSizeBytes())
	}
}

func TestClearResetsCurrentSize(t *testing.T) {
	c := NewFileStateCache(10, 1024)

	c.Set("/a", FileState{Content: "aaa", Timestamp: 1})
	c.Set("/b", FileState{Content: "bbb", Timestamp: 2})

	c.Clear()

	if c.CurrentSizeBytes() != 0 {
		t.Errorf("expected currentSize=0 after clear, got %d", c.CurrentSizeBytes())
	}
}
