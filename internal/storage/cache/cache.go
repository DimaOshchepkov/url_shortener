package cache

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/metrics"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

// URLStorage is the interface that the cache wraps.
// It requires both read and analytics-write operations.
type URLStorage interface {
	GetURL(ctx context.Context, alias string) (string, error)
	IncrementClicks(ctx context.Context, alias string) error
}

// CachedStorage wraps a URLStorage with an in-memory LRU cache.
// On GetURL: checks the cache first (fast path), falls back to the inner storage on miss.
// The cache uses TTL-based expiry — stale entries are lazily evicted.
type CachedStorage struct {
	inner URLStorage
	cache *lru.LRU[string, string]
	log   *slog.Logger

	// metrics: atomics for HitRate(), Prometheus counters for /metrics scraping
	hits   atomic.Uint64
	misses atomic.Uint64
}

// New creates a CachedStorage that wraps the given URLStorage.
// maxSize is the maximum number of cache entries.
// ttl is the time-to-live for each cache entry.
func New(log *slog.Logger, inner URLStorage, maxSize int, ttl time.Duration) *CachedStorage {
	return &CachedStorage{
		inner: inner,
		cache: lru.NewLRU[string, string](maxSize, nil, ttl),
		log:   log.With(slog.String("component", "cache")),
	}
}

// GetURL retrieves a URL by alias, checking the cache first.
func (c *CachedStorage) GetURL(ctx context.Context, alias string) (string, error) {
	// fast path: check cache
	if url, ok := c.cache.Get(alias); ok {
		c.hits.Add(1)
		metrics.CacheHitsTotal.Inc()
		c.log.Debug("cache hit", slog.String("alias", alias))
		return url, nil
	}

	c.misses.Add(1)
	metrics.CacheMissesTotal.Inc()
	c.log.Debug("cache miss", slog.String("alias", alias))

	// slow path: fetch from inner storage
	url, err := c.inner.GetURL(ctx, alias)
	if err != nil {
		return "", err
	}

	// populate cache (best-effort)
	c.cache.Add(alias, url)

	return url, nil
}

// IncrementClicks delegates to the underlying storage.
// Click counts are tracked per-alias in PostgreSQL (business data),
// not in Prometheus (operational metrics).
func (c *CachedStorage) IncrementClicks(ctx context.Context, alias string) error {
	return c.inner.IncrementClicks(ctx, alias)
}

// HitRate returns the cache hit rate as a float between 0 and 1.
// Returns 0 if there have been no requests.
func (c *CachedStorage) HitRate() float64 {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// Len returns the current number of entries in the cache.
func (c *CachedStorage) Len() int {
	return c.cache.Len()
}

// PassThrough wraps a URLStorage without caching — every GetURL call goes directly
// to inner. Implements URLStorage for compatibility with the health handler.
type PassThrough struct {
	inner URLStorage
}

// NewPassThrough creates a PassThrough wrapper.
func NewPassThrough(inner URLStorage) *PassThrough {
	return &PassThrough{inner: inner}
}

func (p *PassThrough) GetURL(ctx context.Context, alias string) (string, error) {
	return p.inner.GetURL(ctx, alias)
}

func (p *PassThrough) IncrementClicks(ctx context.Context, alias string) error {
	return p.inner.IncrementClicks(ctx, alias)
}

func (p *PassThrough) HitRate() float64 { return 0 }

func (p *PassThrough) Len() int { return 0 }
