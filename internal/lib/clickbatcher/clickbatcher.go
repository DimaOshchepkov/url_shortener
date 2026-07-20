package clickbatcher

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Storage is the minimal interface ClickBatcher needs from the underlying DB.
type Storage interface {
	GetURL(ctx context.Context, alias string) (string, error)
	IncrementClicksBy(ctx context.Context, alias string, delta int64) error
}

// ClickBatcher accumulates click increments in memory and flushes them
// to the underlying storage periodically. This reduces database write
// round-trips from one-per-redirect to one-per-alias-per-flush-interval.
type ClickBatcher struct {
	inner    Storage
	log      *slog.Logger
	interval time.Duration

	mu      sync.Mutex
	pending map[string]int64 // alias → count since last flush
}

// New creates a ClickBatcher that wraps the given Storage.
// Flushes happen every interval. Call Run() to start the flush loop.
func New(inner Storage, log *slog.Logger, interval time.Duration) *ClickBatcher {
	return &ClickBatcher{
		inner:    inner,
		log:      log.With(slog.String("component", "clickbatcher")),
		interval: interval,
		pending:  make(map[string]int64),
	}
}

// GetURL delegates directly to the underlying storage — no batching for reads.
func (b *ClickBatcher) GetURL(ctx context.Context, alias string) (string, error) {
	return b.inner.GetURL(ctx, alias)
}

// IncrementClicks records a click in the in-memory buffer.
// The actual database write happens on the next flush.
func (b *ClickBatcher) IncrementClicks(_ context.Context, alias string) error {
	b.mu.Lock()
	b.pending[alias]++
	b.mu.Unlock()
	return nil
}

// Run starts the periodic flush loop. It blocks until ctx is cancelled,
// then performs a final flush before returning.
func (b *ClickBatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	b.log.Info("click batcher started", slog.Duration("interval", b.interval))

	for {
		select {
		case <-ticker.C:
			b.flush(ctx)
		case <-ctx.Done():
			b.log.Info("shutting down, final flush")
			b.flush(context.Background()) // use background ctx so flush isn't cancelled
			return
		}
	}
}

// flush snapshots pending counters and writes them to storage.
func (b *ClickBatcher) flush(ctx context.Context) {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return
	}
	snapshot := b.pending
	b.pending = make(map[string]int64, len(snapshot))
	b.mu.Unlock()

	var total int64
	for alias, count := range snapshot {
		if err := b.inner.IncrementClicksBy(ctx, alias, count); err != nil {
			b.log.Error("flush failed for alias, re-queuing",
				slog.String("alias", alias),
				slog.Int64("count", count),
				slog.String("err", err.Error()),
			)
			// re-queue on failure (best-effort)
			b.mu.Lock()
			b.pending[alias] += count
			b.mu.Unlock()
			continue
		}
		total += count
	}
	if total > 0 {
		b.log.Debug("flushed clicks", slog.Int64("total", total), slog.Int("aliases", len(snapshot)))
	}
}
