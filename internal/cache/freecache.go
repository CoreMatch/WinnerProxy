package cache

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/coocood/freecache"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

const (
	// Key prefixes isolate the two namespaces inside the single
	// freecache instance. HA profiles and Mojang profiles live in
	// the same store but with disjoint key sets.
	keyPrefixHA      = "ha:profile:"
	keyPrefixMojang  = "mojang:hasjoined:"
	// valueOverhead is the approximate fixed bytes per entry that
	// freecache accounts for beyond the raw key+value.
	valueOverhead = 64
)

// FreeCache is a ProfileCache backed by a freecache.Cache instance.
// Values are JSON-encoded hrpauth.PlayerProfile objects.
type FreeCache struct {
	c          *freecache.Cache
	ttlSeconds int
}

// NewFreeCache builds a FreeCache with sizeBytes total memory and the
// given TTL. freecache runs its own background expiration sweep at a
// fixed 30s interval; entries also expire lazily on read.
func NewFreeCache(sizeBytes int, ttl time.Duration) *FreeCache {
	if sizeBytes <= 0 {
		return nil
	}
	c := freecache.NewCache(sizeBytes)
	ttlSec := int(ttl.Seconds())
	if ttlSec <= 0 {
		ttlSec = 300
	}
	log.Printf("profile cache: freecache %d bytes, ttl=%ds", sizeBytes, ttlSec)
	return &FreeCache{c: c, ttlSeconds: ttlSec}
}

// ApproxEntrySize returns the approximate per-entry footprint (key
// + value + valueOverhead). Used by tests / metrics.
func (f *FreeCache) ApproxEntrySize(key, value []byte) int {
	return len(key) + len(value) + valueOverhead
}

func haKey(uuid string) []byte {
	// Pre-size to avoid an extra alloc from Sprintf.
	k := make([]byte, 0, len(keyPrefixHA)+len(uuid))
	k = append(k, keyPrefixHA...)
	k = append(k, uuid...)
	return k
}

func mojangKey(username string) []byte {
	k := make([]byte, 0, len(keyPrefixMojang)+len(username))
	k = append(k, keyPrefixMojang...)
	k = append(k, username...)
	return k
}

// GetHAProfile implements ProfileCache.
func (f *FreeCache) GetHAProfile(uuid string) (*hrpauth.PlayerProfile, bool) {
	if f == nil {
		return nil, false
	}
	v, err := f.c.Get(haKey(uuid))
	if err != nil {
		// err is ErrEntryNotFound on miss; any other error is also
		// treated as a miss for safety.
		return nil, false
	}
	var p hrpauth.PlayerProfile
	if err := json.Unmarshal(v, &p); err != nil {
		log.Printf("profile cache: decode HA profile uuid=%s: %v", uuid, err)
		return nil, false
	}
	return &p, true
}

// SetHAProfile implements ProfileCache.
func (f *FreeCache) SetHAProfile(uuid string, p *hrpauth.PlayerProfile) error {
	if f == nil || p == nil {
		return nil
	}
	v, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := f.c.Set(haKey(uuid), v, f.ttlSeconds); err != nil {
		// ErrLargeEntry (entry > 1/1024 of cache size) is non-fatal:
		// the next request will simply re-query the upstream.
		if !errors.Is(err, freecache.ErrLargeEntry) {
			log.Printf("profile cache: set HA uuid=%s: %v", uuid, err)
		}
		return err
	}
	return nil
}

// GetMojangProfile implements ProfileCache.
func (f *FreeCache) GetMojangProfile(username string) (*hrpauth.PlayerProfile, bool) {
	if f == nil {
		return nil, false
	}
	v, err := f.c.Get(mojangKey(username))
	if err != nil {
		return nil, false
	}
	var p hrpauth.PlayerProfile
	if err := json.Unmarshal(v, &p); err != nil {
		log.Printf("profile cache: decode Mojang profile name=%s: %v", username, err)
		return nil, false
	}
	return &p, true
}

// SetMojangProfile implements ProfileCache.
func (f *FreeCache) SetMojangProfile(username string, p *hrpauth.PlayerProfile) error {
	if f == nil || p == nil {
		return nil
	}
	v, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := f.c.Set(mojangKey(username), v, f.ttlSeconds); err != nil {
		if !errors.Is(err, freecache.ErrLargeEntry) {
			log.Printf("profile cache: set Mojang name=%s: %v", username, err)
		}
		return err
	}
	return nil
}
