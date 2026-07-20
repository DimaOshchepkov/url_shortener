// Package metrics defines and registers Prometheus metrics for the URL shortener.
//
// Design decisions:
//   - Package-level vars with promauto are the standard Go pattern for Prometheus.
//     Registration happens at import time via promauto's init-registration.
//   - Per-alias labels are deliberately avoided to prevent cardinality explosion.
//     Per-link analytics (click counts) are stored in PostgreSQL, not Prometheus.
//   - The redirect handler and cache import this package; metrics are available at
//     the /metrics endpoint served on the observability port (:6060).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "url_shortener"
	subsystem = "redirect"
)

var (
	// RedirectRequestsTotal counts every redirect request partitioned by outcome.
	// Labels:
	//   status — "success" (302 sent), "not_found" (alias unknown), "error" (internal failure)
	RedirectRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "requests_total",
		Help:      "Total number of redirect requests by status",
	}, []string{"status"})

	// RedirectDurationSeconds measures how long the full redirect handler takes.
	// This includes alias resolution, click count increment, and response write.
	// The histogram uses default Prometheus buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
	RedirectDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "duration_seconds",
		Help:      "Duration of redirect requests in seconds",
		Buckets:   prometheus.DefBuckets,
	})

	// CacheHitsTotal counts successful cache lookups for alias->URL resolution.
	CacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "url_shortener",
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "Total number of cache hits",
	})

	// CacheMissesTotal counts cache lookups that fell through to the underlying storage.
	CacheMissesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "url_shortener",
		Subsystem: "cache",
		Name:      "misses_total",
		Help:      "Total number of cache misses",
	})
)
