package media

import (
	"fmt"
	"sync"
	"time"
)

type pageCache struct {
	data        []byte
	contentType string
}

type prefetchCache struct {
	mu    sync.Mutex
	pages map[string]pageCache // key: "archivePath:pageIndex"
	times map[string]time.Time
}

var pfc = &prefetchCache{
	pages: make(map[string]pageCache),
	times: make(map[string]time.Time),
}

func init() {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			pfc.evict()
		}
	}()
}

func (c *prefetchCache) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, t := range c.times {
		if now.Sub(t) > 2*time.Minute {
			delete(c.pages, k)
			delete(c.times, k)
		}
	}
}

func cacheKey(archivePath string, pageIndex int) string {
	return fmt.Sprintf("%s:%d", archivePath, pageIndex)
}

func (c *prefetchCache) get(archivePath string, pageIndex int) ([]byte, string, bool) {
	key := cacheKey(archivePath, pageIndex)
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.pages[key]; ok {
		c.times[key] = time.Now()
		return p.data, p.contentType, true
	}
	return nil, "", false
}

func (c *prefetchCache) set(archivePath string, pageIndex int, data []byte, ct string) {
	key := cacheKey(archivePath, pageIndex)
	c.mu.Lock()
	defer c.mu.Unlock()
	// Limit cache to ~10 entries to keep memory low
	if len(c.pages) > 10 {
		// Evict oldest
		var oldestKey string
		var oldestTime time.Time
		for k, t := range c.times {
			if oldestKey == "" || t.Before(oldestTime) {
				oldestKey = k
				oldestTime = t
			}
		}
		if oldestKey != "" {
			delete(c.pages, oldestKey)
			delete(c.times, oldestKey)
		}
	}
	c.pages[key] = pageCache{data: data, contentType: ct}
	c.times[key] = time.Now()
}

// PrefetchAhead prefetches the next few pages in background (useful for RAR).
func PrefetchAhead(archivePath string, currentPage, totalPages int) {
	for i := 1; i <= 3; i++ {
		next := currentPage + i
		if next >= totalPages {
			break
		}
		if _, _, ok := pfc.get(archivePath, next); ok {
			continue // already cached
		}
		go func(idx int) {
			data, ct, err := getPageDirect(archivePath, idx)
			if err == nil {
				pfc.set(archivePath, idx, data, ct)
			}
		}(next)
	}
}

// GetPageCached tries the cache first, then falls back to direct extraction.
func GetPageCached(archivePath string, pageIndex int) ([]byte, string, error) {
	if data, ct, ok := pfc.get(archivePath, pageIndex); ok {
		return data, ct, nil
	}
	return getPageDirect(archivePath, pageIndex)
}

// getPageDirect extracts a page without cache (calls the format-specific function).
func getPageDirect(archivePath string, pageIndex int) ([]byte, string, error) {
	return GetPage(archivePath, pageIndex)
}
