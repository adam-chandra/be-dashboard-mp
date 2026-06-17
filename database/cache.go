package database

import (
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// SimpleCache adalah L1 in-memory key/value cache dengan TTL per-entry.
// Janitor goroutine bersih-bersih entry kadaluarsa tiap 2*ttl.
type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

type SimpleCache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
	ttl   time.Duration
}

func NewSimpleCache(ttl time.Duration) *SimpleCache {
	c := &SimpleCache{
		store: make(map[string]cacheEntry),
		ttl:   ttl,
	}
	go c.janitor()
	return c
}

func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	e, ok := c.store[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.store, key)
		c.mu.Unlock()
		return nil, false
	}
	return e.value, true
}

func (c *SimpleCache) Set(key string, val interface{}) {
	c.SetWithTTL(key, val, c.ttl)
}

func (c *SimpleCache) SetWithTTL(key string, val interface{}, ttl time.Duration) {
	c.mu.Lock()
	c.store[key] = cacheEntry{value: val, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *SimpleCache) Invalidate() {
	c.mu.Lock()
	c.store = make(map[string]cacheEntry)
	c.mu.Unlock()
}

func (c *SimpleCache) janitor() {
	ticker := time.NewTicker(2 * c.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.store {
			if now.After(e.expiresAt) {
				delete(c.store, k)
			}
		}
		c.mu.Unlock()
	}
}

// AnalyticsCache: L1 in-memory cache. TTL default 60s, override via CACHE_TTL_SECONDS.
var AnalyticsCache = NewSimpleCache(loadCacheTTL())

// sfGroup men-dedupe loader concurrent untuk key yang sama → mencegah
// "thundering herd" (banyak request identik memicu N kali query mahal saat miss).
var sfGroup singleflight.Group

func loadCacheTTL() time.Duration {
	if v := os.Getenv("CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

func CacheTTL() time.Duration {
	return loadCacheTTL()
}

// Cached: helper generic read-through L1 → L2 (Redis) → loader.
// Memakai singleflight: request concurrent dengan key sama hanya menjalankan
// loader satu kali, sisanya menunggu hasil yang sama.
//
// Pola dipakai oleh semua endpoint analitik (metrik/grafik/peringkat/dst).
// Tipe T harus serializable JSON (struct/slice/map biasa).
func Cached[T any](key string, loader func() (T, error)) (T, error) {
	return CachedTTL[T](key, CacheTTL(), loader)
}

// CachedTTL sama dengan Cached tapi TTL kustom. Cocok untuk data yang
// jarang berubah seperti dropdown options (TTL panjang) atau geo mapping.
func CachedTTL[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var zero T

	if v, ok := AnalyticsCache.Get(key); ok {
		if r, ok2 := v.(T); ok2 {
			return r, nil
		}
	}

	var fromRedis T
	if hit, _ := RedisGetJSON(key, &fromRedis); hit {
		AnalyticsCache.SetWithTTL(key, fromRedis, ttl)
		return fromRedis, nil
	}

	v, err, _ := sfGroup.Do(key, func() (interface{}, error) {
		res, err := loader()
		if err != nil {
			return zero, err
		}
		AnalyticsCache.SetWithTTL(key, res, ttl)
		RedisSetJSON(key, res, ttl)
		return res, nil
	})
	if err != nil {
		return zero, err
	}
	if r, ok := v.(T); ok {
		return r, nil
	}
	return zero, nil
}

// InvalidateAll bersihkan L1 + L2 (semua prefix yang dipakai aplikasi).
func InvalidateAll() {
	AnalyticsCache.Invalidate()
	for _, p := range []string{"metrik:", "grafik:", "peringkat:", "logistik:", "options:", "ringkasan:", "sales:"} {
		RedisInvalidatePrefix(p)
	}
}
