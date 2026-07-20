package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubStorage implements URLStorage for testing.
type stubStorage struct {
	urls        map[string]string
	clicks      map[string]int64
	getURLCalls int // track slow-path calls
}

func newStubStorage() *stubStorage {
	return &stubStorage{
		urls:   make(map[string]string),
		clicks: make(map[string]int64),
	}
}

func (s *stubStorage) GetURL(_ context.Context, alias string) (string, error) {
	s.getURLCalls++
	url, ok := s.urls[alias]
	if !ok {
		return "", errors.New("not found")
	}
	return url, nil
}

func (s *stubStorage) IncrementClicks(_ context.Context, alias string) error {
	s.clicks[alias]++
	return nil
}

func TestCachedStorage_CacheHit(t *testing.T) {
	inner := newStubStorage()
	inner.urls["abc123"] = "https://example.com"

	cs := New(testLogger, inner, 100, 5*time.Minute)
	ctx := context.Background()

	// First call — cache miss, should hit inner storage.
	url1, err := cs.GetURL(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url1)
	assert.Equal(t, 1, inner.getURLCalls)

	// Second call — cache hit, should NOT hit inner storage.
	url2, err := cs.GetURL(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url2)
	assert.Equal(t, 1, inner.getURLCalls) // still 1

	// Check hit rate: 1 hit, 1 miss = 50%.
	assert.InDelta(t, 0.5, cs.HitRate(), 0.01)
	assert.Equal(t, 1, cs.Len())
}

func TestCachedStorage_CacheMiss(t *testing.T) {
	inner := newStubStorage()

	cs := New(testLogger, inner, 100, 5*time.Minute)
	ctx := context.Background()

	_, err := cs.GetURL(ctx, "nonexistent")
	require.Error(t, err)
	assert.Equal(t, 1, inner.getURLCalls)
	assert.Equal(t, float64(0), cs.HitRate())
	assert.Equal(t, 0, cs.Len())
}

func TestCachedStorage_CacheExpiry(t *testing.T) {
	inner := newStubStorage()
	inner.urls["abc123"] = "https://example.com"

	// Very short TTL for testing expiry.
	cs := New(testLogger, inner, 100, 10*time.Millisecond)
	ctx := context.Background()

	// Populate cache.
	_, err := cs.GetURL(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, 1, cs.Len())

	// Wait for expiry.
	time.Sleep(20 * time.Millisecond)

	// Should be a miss now — entry expired.
	_, err = cs.GetURL(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, 2, inner.getURLCalls) // second slow-path call
}

func TestCachedStorage_IncrementClicks(t *testing.T) {
	inner := newStubStorage()

	cs := New(testLogger, inner, 100, 5*time.Minute)
	ctx := context.Background()

	err := cs.IncrementClicks(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, int64(1), inner.clicks["abc123"])
}

func TestPassThrough_GetURL(t *testing.T) {
	inner := newStubStorage()
	inner.urls["abc123"] = "https://example.com"

	pt := NewPassThrough(inner)
	ctx := context.Background()

	url, err := pt.GetURL(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)

	// PassThrough always returns 0 hit rate.
	assert.Equal(t, float64(0), pt.HitRate())
	assert.Equal(t, 0, pt.Len())
}

func TestPassThrough_IncrementClicks(t *testing.T) {
	inner := newStubStorage()
	pt := NewPassThrough(inner)
	ctx := context.Background()

	err := pt.IncrementClicks(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, int64(1), inner.clicks["abc123"])
}
