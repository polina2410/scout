package thumb

import (
	"sync"
	"testing"
)

func newTestCache(t *testing.T, maxSize int64) *DiskCache {
	t.Helper()
	c, err := NewDiskCache(t.TempDir(), maxSize)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	return c
}

func TestDiskCache_PutGet(t *testing.T) {
	c := newTestCache(t, 1024)
	if err := c.Put("key", []byte("hello")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	data, ok := c.Get("key")
	if !ok {
		t.Fatal("Get returned false after Put")
	}
	if string(data) != "hello" {
		t.Errorf("data: got %q, want %q", data, "hello")
	}
}

func TestDiskCache_Miss(t *testing.T) {
	c := newTestCache(t, 1024)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get should return false for unknown key")
	}
}

func TestDiskCache_LRUEviction(t *testing.T) {
	// maxSize = 10 bytes; "a"=6B, "b"=6B → inserting "b" must evict "a"
	c := newTestCache(t, 10)

	if err := c.Put("a", []byte("aaaaaa")); err != nil { // 6 bytes
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Put("b", []byte("bbbbbb")); err != nil { // 6 bytes → evicts "a"
		t.Fatalf("Put b: %v", err)
	}

	if _, ok := c.Get("a"); ok {
		t.Error("'a' should have been evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("'b' should still be present")
	}
}

func TestDiskCache_UpdateMovesToFront(t *testing.T) {
	// Put "a", Put "b"; Get "a" (moves to front); Put "c" (forces eviction of "b" — LRU)
	c := newTestCache(t, 15)

	if err := c.Put("a", []byte("aaaaaa")); err != nil { // 6 bytes
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Put("b", []byte("bbbbbb")); err != nil { // 6 bytes; total=12
		t.Fatalf("Put b: %v", err)
	}

	// Access "a" — promotes it to front of LRU; "b" becomes LRU tail
	if _, ok := c.Get("a"); !ok {
		t.Fatal("'a' should be present before eviction test")
	}

	// Insert "c" (6B): total would be 18 > 15; must evict "b" (LRU tail)
	if err := c.Put("c", []byte("cccccc")); err != nil {
		t.Fatalf("Put c: %v", err)
	}

	if _, ok := c.Get("b"); ok {
		t.Error("'b' should have been evicted (it was LRU)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("'a' should still be present (it was recently accessed)")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("'c' should be present")
	}
}

func TestDiskCache_WarmFromDisk(t *testing.T) {
	dir := t.TempDir()

	// First instance writes two entries to disk.
	c1, err := NewDiskCache(dir, 1024)
	if err != nil {
		t.Fatalf("NewDiskCache c1: %v", err)
	}
	if err := c1.Put("a", []byte("aaaaaa")); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c1.Put("b", []byte("bbbbbb")); err != nil {
		t.Fatalf("Put b: %v", err)
	}

	// A fresh instance over the same dir must see the entries as warm hits,
	// not cold misses — that's the restart cold-start fix.
	c2, err := NewDiskCache(dir, 1024)
	if err != nil {
		t.Fatalf("NewDiskCache c2: %v", err)
	}
	if entries, total := c2.Stats(); entries != 2 || total != 12 {
		t.Errorf("warmed stats: got entries=%d total=%d, want 2 and 12", entries, total)
	}
	if data, ok := c2.Get("a"); !ok || string(data) != "aaaaaa" {
		t.Errorf("warmed Get(a): got %q ok=%v, want \"aaaaaa\" true", data, ok)
	}
}

func TestDiskCache_WarmEvictsOverCap(t *testing.T) {
	dir := t.TempDir()

	// Fill the dir under a generous cap...
	c1, err := NewDiskCache(dir, 1024)
	if err != nil {
		t.Fatalf("NewDiskCache c1: %v", err)
	}
	if err := c1.Put("a", []byte("aaaaaa")); err != nil { // 6 bytes
		t.Fatalf("Put a: %v", err)
	}
	if err := c1.Put("b", []byte("bbbbbb")); err != nil { // 6 bytes; total=12
		t.Fatalf("Put b: %v", err)
	}

	// ...then reopen with a smaller cap that only fits one entry.
	c2, err := NewDiskCache(dir, 10)
	if err != nil {
		t.Fatalf("NewDiskCache c2: %v", err)
	}
	if entries, total := c2.Stats(); entries != 1 || total != 6 {
		t.Errorf("warmed-evicted stats: got entries=%d total=%d, want 1 and 6", entries, total)
	}
}

func TestDiskCache_ConcurrentAccess(t *testing.T) {
	c := newTestCache(t, 1024*1024)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := "key" + string(rune('A'+i%5))
			_ = c.Put(key, []byte("data"))
			_, _ = c.Get(key)
		}()
	}
	wg.Wait()
	// If there's a data race, -race will report it. Just confirm no panic.
}
