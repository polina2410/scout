package thumb

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DiskCache is a thread-safe, size-bounded LRU disk cache.
// Entries are files on disk; an in-memory index tracks sizes for eviction.
type DiskCache struct {
	dir     string
	maxSize int64
	mu      sync.Mutex
	index   map[string]*cacheEntry
	lru     list.List // front = most recently used
	total   int64
}

type cacheEntry struct {
	key  string
	size int64
	el   *list.Element
}

// NewDiskCache creates (or reuses) a cache directory rooted at dir with a maxSize byte cap.
func NewDiskCache(dir string, maxSize int64) (*DiskCache, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve cache dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &DiskCache{
		dir:     abs,
		maxSize: maxSize,
		index:   make(map[string]*cacheEntry),
	}, nil
}

// safePath returns the resolved path for key within the cache directory.
// Returns an error if the key would escape the cache dir (path traversal guard).
func (c *DiskCache) safePath(key string) (string, error) {
	p := filepath.Join(c.dir, filepath.Clean(key))
	if !strings.HasPrefix(p, c.dir+string(filepath.Separator)) && p != c.dir {
		return "", fmt.Errorf("cache key %q escapes cache directory", key)
	}
	return p, nil
}

// Get returns the cached bytes for key and true, or nil, false on a miss.
// On hit, the entry is moved to the front of the LRU list.
func (c *DiskCache) Get(key string) ([]byte, bool) {
	path, err := c.safePath(key)
	if err != nil {
		return nil, false
	}

	c.mu.Lock()
	e, ok := c.index[key]
	if !ok {
		c.mu.Unlock()
		return nil, false
	}
	// Update LRU position while holding the lock so eviction order stays consistent.
	c.lru.MoveToFront(e.el)
	c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		// File missing or corrupt — remove the stale index entry.
		c.mu.Lock()
		if cur, still := c.index[key]; still && cur == e {
			c.lru.Remove(e.el)
			c.total -= e.size
			delete(c.index, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return data, true
}

// Put stores data under key, evicting LRU entries as needed to respect maxSize.
func (c *DiskCache) Put(key string, data []byte) error {
	path, err := c.safePath(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	size := int64(len(data))
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already cached, remove old entry so we can re-add it at the front.
	if old, ok := c.index[key]; ok {
		c.lru.Remove(old.el)
		c.total -= old.size
		delete(c.index, key)
	}

	// Evict from the back until the new entry fits.
	for c.total+size > c.maxSize && c.lru.Len() > 0 {
		back := c.lru.Back()
		if back == nil {
			break
		}
		victim := back.Value.(*cacheEntry)
		c.lru.Remove(back)
		c.total -= victim.size
		delete(c.index, victim.key)
		os.Remove(filepath.Join(c.dir, victim.key)) //nolint:errcheck
	}

	e := &cacheEntry{key: key, size: size}
	e.el = c.lru.PushFront(e)
	c.index[key] = e
	c.total += size
	return nil
}

// Stats returns a snapshot of the current cache state.
func (c *DiskCache) Stats() (entries int, totalBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.index), c.total
}
