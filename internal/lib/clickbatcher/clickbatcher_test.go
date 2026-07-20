package clickbatcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubStorage implements Storage for testing.
type stubStorage struct {
	mu              sync.Mutex
	urls            map[string]string
	clicks          map[string]int64
	incrementErrors map[string]error // alias → error to return
}

func newStubStorage() *stubStorage {
	return &stubStorage{
		urls:            make(map[string]string),
		clicks:          make(map[string]int64),
		incrementErrors: make(map[string]error),
	}
}

func (s *stubStorage) GetURL(_ context.Context, alias string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.urls[alias], nil
}

func (s *stubStorage) IncrementClicksBy(_ context.Context, alias string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.incrementErrors[alias]; ok {
		return err
	}
	s.clicks[alias] += delta
	return nil
}

func (s *stubStorage) clicksFor(alias string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clicks[alias]
}

func TestClickBatcher_GetURLDelegates(t *testing.T) {
	inner := newStubStorage()
	inner.urls["abc"] = "https://example.com"

	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	url, err := b.GetURL(ctx, "abc")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
}

func TestClickBatcher_IncrementBuffers(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	// Increment should NOT write to storage immediately.
	err := b.IncrementClicks(ctx, "abc")
	require.NoError(t, err)
	assert.Equal(t, int64(0), inner.clicksFor("abc")) // still 0 in DB

	// After flush, clicks should be written.
	b.flush(ctx)
	assert.Equal(t, int64(1), inner.clicksFor("abc"))
}

func TestClickBatcher_FlushAggregates(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	// Multiple increments on the same alias.
	for i := 0; i < 5; i++ {
		err := b.IncrementClicks(ctx, "abc")
		require.NoError(t, err)
	}

	// Flush should aggregate into one IncrementClicksBy(alias, 5) call.
	b.flush(ctx)
	assert.Equal(t, int64(5), inner.clicksFor("abc"))
}

func TestClickBatcher_MultipleAliases(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	b.IncrementClicks(ctx, "abc")
	b.IncrementClicks(ctx, "abc")
	b.IncrementClicks(ctx, "xyz")

	b.flush(ctx)

	assert.Equal(t, int64(2), inner.clicksFor("abc"))
	assert.Equal(t, int64(1), inner.clicksFor("xyz"))
}

func TestClickBatcher_FlushOnEmpty(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	// Flushing with no pending should not panic.
	b.flush(ctx)
	assert.Equal(t, int64(0), inner.clicksFor("abc"))
}

func TestClickBatcher_GracefulShutdownFlushes(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the batcher in background.
	done := make(chan struct{})
	go func() {
		b.Run(ctx)
		close(done)
	}()

	// Buffer some clicks.
	b.IncrementClicks(ctx, "abc")

	// Cancel immediately — should trigger final flush.
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("Run() did not return within 1 second")
	}

	assert.Equal(t, int64(1), inner.clicksFor("abc"))
}

func TestClickBatcher_RequeueOnFailure(t *testing.T) {
	inner := newStubStorage()
	inner.incrementErrors["abc"] = assert.AnError
	b := New(inner, testLogger, time.Second)
	ctx := context.Background()

	b.IncrementClicks(ctx, "abc")

	// First flush fails — clicks should be re-queued.
	b.flush(ctx)

	b.mu.Lock()
	pending := b.pending["abc"]
	b.mu.Unlock()
	assert.Equal(t, int64(1), pending, "clicks should be re-queued on failure")
}

func TestClickBatcher_RunPeriodicFlush(t *testing.T) {
	inner := newStubStorage()
	b := New(inner, testLogger, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go b.Run(ctx)

	b.IncrementClicks(ctx, "abc")

	// Wait for a periodic flush.
	time.Sleep(50 * time.Millisecond)
	cancel()

	assert.GreaterOrEqual(t, inner.clicksFor("abc"), int64(1))
}
