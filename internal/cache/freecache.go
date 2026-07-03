package cache

import (
	"errors"
	"time"

	"github.com/coocood/freecache"
)

// ErrCacheMiss is returned when a key is not present in the cache.
var ErrCacheMiss = errors.New("cache: key not found")

// Cache is a thin wrapper around freecache exposing a small, intention-revealing API.
type Cache struct {
	c *freecache.Cache
}

// New creates a Cache with the given size in bytes.
// freecache enforces a minimum of 512KB and silently rounds up smaller values.
func New(size int) *Cache {
	return &Cache{c: freecache.NewCache(size)}
}

// Set stores value under key with the given TTL.
// expireSeconds == 0 means "never expire".
func (c *Cache) Set(key, value []byte, expireSeconds int) error {
	return c.c.Set(key, value, expireSeconds)
}

// Get fetches the value for key. It returns ErrCacheMiss when the key is absent.
func (c *Cache) Get(key []byte) ([]byte, error) {
	v, err := c.c.Get(key)
	if err != nil {
		if errors.Is(err, freecache.ErrNotFound) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	return v, nil
}

// Delete removes the entry for key. It returns whether a key was actually removed.
func (c *Cache) Delete(key []byte) bool {
	return c.c.Del(key)
}

// SetWithTTL is a convenience helper that accepts a time.Duration.
func (c *Cache) SetWithTTL(key, value []byte, ttl time.Duration) error {
	seconds := int(ttl.Seconds())
	if ttl > 0 && seconds == 0 {
		seconds = 1
	}
	return c.c.Set(key, value, seconds)
}

// HitRate returns the cache hit ratio (hits / (hits + misses)).
// Returns 0 when there have been no lookups.
func (c *Cache) HitRate() float64 {
	return c.c.HitRate()
}

// Stats reports the current cache counters.
type Stats struct {
	HitCount     int64   `json:"hit_count"`
	MissCount    int64   `json:"miss_count"`
	LookupCount  int64   `json:"lookup_count"`
	HitRate      float64 `json:"hit_rate"`
	EntryCount   int64   `json:"entry_count"`
	OverwriteCnt int64   `json:"overwrite_count"`
	EvacuateCnt  int64   `json:"evacuate_count"`
	ExpiredCnt   int64   `json:"expired_count"`
}

// Stats returns a snapshot of cache statistics.
func (c *Cache) Stats() Stats {
	return Stats{
		HitCount:     c.c.HitCount(),
		MissCount:    c.c.MissCount(),
		LookupCount:  c.c.LookupCount(),
		HitRate:      c.c.HitRate(),
		EntryCount:   c.c.EntryCount(),
		OverwriteCnt: c.c.OverwriteCount(),
		EvacuateCnt:  c.c.EvacuateCount(),
		ExpiredCnt:   c.c.ExpiredCount(),
	}
}
