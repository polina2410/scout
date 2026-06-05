package thumb

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// validCacheKey matches the only shape a cache key ever takes:
// {photoId}_{w}_{dpr}_{fmt}, all of which are hex, digits, hyphens or letters.
// Anything else (path separators, dots, ":", control chars) is rejected.
var validCacheKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// tmpPrefix marks in-progress cache writes so they can be skipped on load and
// never collide with a real cache key (keys never start with a dot).
const tmpPrefix = ".tmp-"

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
	c := &DiskCache{
		dir:     abs,
		maxSize: maxSize,
		index:   make(map[string]*cacheEntry),
	}
	// Rebuild the LRU index from any thumbnails left on disk by a prior run so
	// restarts don't treat a full cache directory as cold (every request a miss).
	c.warm()
	return c, nil
}

// warm scans the cache directory and repopulates the in-memory index from files
// already on disk, so a restart reuses a warm cache instead of regenerating
// everything. Files are ordered oldest-first by mtime so the most recently
// written end up at the front of the LRU. Best-effort: a scan failure simply
// leaves the cache cold rather than blocking startup.
func (c *DiskCache) warm() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}

	type fileInfo struct {
		name string
		size int64
		mod  int64
	}
	files := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Skip temp files left behind by an interrupted Put; they are not valid
		// cache entries and would otherwise be indexed under a bogus key.
		if strings.HasPrefix(e.Name(), tmpPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // file vanished between ReadDir and Info — skip it.
		}
		files = append(files, fileInfo{name: e.Name(), size: info.Size(), mod: info.ModTime().UnixNano()})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range files {
		e := &cacheEntry{key: f.name, size: f.size}
		e.el = c.lru.PushFront(e)
		c.index[f.name] = e
		c.total += f.size
	}
	// A prior run may have used a larger cap; evict down to the current maxSize.
	for c.total > c.maxSize && c.lru.Len() > 0 {
		back := c.lru.Back()
		victim := back.Value.(*cacheEntry)
		c.lru.Remove(back)
		c.total -= victim.size
		delete(c.index, victim.key)
		os.Remove(filepath.Join(c.dir, victim.key)) //nolint:errcheck
	}
}

// safePath returns the resolved path for key within the cache directory.
// The allowlist regexp rejects any key containing a path separator, "..", or
// other unexpected characters, so a key can never escape the cache dir and the
// guard does not depend on filesystem case-sensitivity.
func (c *DiskCache) safePath(key string) (string, error) {
	if !validCacheKey.MatchString(key) {
		return "", fmt.Errorf("invalid cache key %q", key)
	}
	return filepath.Join(c.dir, key), nil
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

	// Write to a temp file in the same directory, then atomically rename it into
	// place. This ensures a concurrent Get never reads a half-written file, and
	// two concurrent Puts for the same key can't interleave partial writes — the
	// last rename wins cleanly. (Singleflight makes this rare, but the cache must
	// be correct even if a caller bypasses it.)
	tmp, err := os.CreateTemp(c.dir, tmpPrefix+"*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()        //nolint:errcheck
		os.Remove(tmpName) //nolint:errcheck
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()        //nolint:errcheck
		os.Remove(tmpName) //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName) //nolint:errcheck
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName) //nolint:errcheck
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
