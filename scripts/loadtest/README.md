# Load Test Protocol: URL Shortener Redirect Performance

A systematic bottleneck investigation using k6 and pprof.

## Tools

| Tool | Purpose |
|------|---------|
| [k6](https://grafana.com/docs/k6/) | Load generator: JS scenarios, percentiles (p50/p90/p95/p99), JSON export |
| `net/http/pprof` | Go built-in CPU/memory profiler on port `:6060` |
| `compare.py` | Side-by-side comparison with improvement percentages |
| Docker Compose | Isolated environment: app + PostgreSQL |

## Data & Scenario

**Setup:** 500 aliases via `setup.sh` → `/tmp/aliases.json`

**Traffic:** 80% hot (top 10% of aliases) / 20% cold — Zipf distribution

**Load stages:** 50 → 200 → 500 → 1000 → 2000 VUs over 2m20s

**Thresholds:** p95 < 500ms, error rate < 0.1%, check rate > 99%

## How We Profiled

```bash
# 1. Start load test
k6 run --summary-export=results.json scripts/loadtest/redirect.js

# 2. During peak load (30-80s in), capture 30s CPU profile from another terminal:
curl -s -o cpu.pprof 'http://localhost:6060/debug/pprof/profile?seconds=30'

# 3. Analyze
go tool pprof -http=:8081 cpu.pprof    # interactive flame graph
go tool pprof -top cpu.pprof            # top functions
```

## Experiments & Results

### Round 1: Baseline (no cache)

**Setup:** `CACHE_ENABLED=false`, logging applied globally via `mwLogger`

**Hypothesis:** bottleneck = PostgreSQL

### Round 2: In-Memory Cache

**Setup:** `CACHE_ENABLED=true`, LRU 10K entries, 5min TTL, cache-aside pattern

**Hypothesis:** hot aliases served from memory, p50 drops, bottleneck shifts to CPU

```
======================================================================
  LOAD TEST COMPARISON: No Cache vs With Cache
======================================================================

  Metric                   No Cache   With Cache    Improvement
  -------------------- ------------ ------------ --------------
  Requests/sec              3842.8/s      4204.3/s          +9.4%
  Error rate                  0.00%        0.00%         +0.00pp
  p50 (median)               36.37ms       12.26ms         +66.3%
  p90 latency               177.16ms      158.36ms         +10.6%
  p95 latency               220.40ms      209.22ms          +5.1%
  p99 latency               325.02ms      322.05ms          +0.9%
  Avg latency                67.80ms       51.45ms         +24.1%
  Max latency               614.29ms      634.94ms          -3.4%

  Total requests: 538,279 (no cache) vs 588,833 (with cache)
======================================================================

  Cache hit rate: 99.9%
```

**Observation:** p50 dropped 66% (cache works). p95 barely moved (+5%). Why?

### pprof Finding: Logging Consumes 25% CPU

A 30s CPU profile during Round 2 revealed:

```
slog.(*Logger).log          24.71%  ← log.Info() call from middleware
slog.(*commonHandler).handle 20.76%  ← JSONHandler marshaling
bufio.(*Writer).Flush        20.37%  ← synchronous write to stderr
```

`internal/transport/middleware/logger/logger.go:31` logs every request:

```go
entry.Info("request completed", status, bytes, duration, ...)
```

At 4,200 RPS this means 4,200 JSON marshal + write operations per second — about 25% of total CPU time. This overhead hits **all requests equally**, which explains why p95/p99 barely improved in Round 2.

### Round 3: Disable Logging for Redirect

**Setup:** `CACHE_ENABLED=true`, `mwLogger` removed from `GET /{alias}` route. Logging preserved for other endpoints (`/url`, `/user`).

**Change** (`internal/app/app.go`):

```go
// Before: mwLogger applied globally
router.Use(mwLogger.New(log))

// After: mwLogger only on non-redirect routes
router.Route("/url", func(r chi.Router) {
    r.Use(mwLogger.New(log))   // logging for API routes
    ...
})
router.Get("/{alias}", urlRed.New(log, facade))  // no logging
```

**Hypothesis:** removing 25% CPU overhead reduces latency across all percentiles.

```
======================================================================
  LOAD TEST COMPARISON: With Cache (logged) vs With Cache (no log)
======================================================================

  Metric                   Logged      No Log       Improvement
  -------------------- ------------ ------------ --------------
  Requests/sec              4204.3/s      4542.1/s          +8.0%
  Error rate                  0.00%        0.00%         +0.00pp
  p50 (median)               12.26ms        8.90ms         +27.4%
  p90 latency               158.36ms      120.65ms         +23.8%
  p95 latency               209.22ms      164.33ms         +21.5%
  p99 latency               322.05ms      256.01ms         +20.5%
  Avg latency                51.45ms       38.89ms         +24.4%
  Max latency               634.94ms      610.61ms          +3.8%

  Total requests: 588,833 (logged) vs 636,135 (no log)
======================================================================
```

### Full Journey (R1 → R3)

| Metric | R1 (No Cache) | R2 (+Cache) | R3 (−Log) | **Total Improvement** |
|--------|:---:|:---:|:---:|:---:|
| Throughput | 3,843/s | 4,204/s | 4,542/s | **+18.2%** |
| p50 | 36.37ms | 12.26ms | 8.90ms | **−75.5%** |
| p95 | 220.40ms | 209.22ms | 164.33ms | **−25.4%** |
| p99 | 325.02ms | 322.05ms | 256.01ms | **−21.2%** |
| Avg | 67.80ms | 51.45ms | 38.89ms | **−42.6%** |

## Bottleneck Hierarchy (Final)

```
PostgreSQL ──→ Cache (✅ −66% p50) ──→ slog JSON logging (✅ −28% p50) ──→ CPU/Network (~30% syscalls)
```

## Key Takeaways

1. **Cache-aside works**: 66% p50 improvement with zero handler changes
2. **pprof is essential**: without it, 25% CPU going to logging is invisible
3. **Logging has a cost**: structured JSON logging at 4K+ RPS = 25% CPU. Skip it on high-traffic endpoints, keep it on operational routes (/url, /user)
4. **Bottleneck discovery is iterative**: each fix reveals the next layer

## Quick Start

```bash
# 1. Generate data
./scripts/loadtest/setup.sh 500

# 2. Toggle cache in config.env: CACHE_ENABLED=true|false
docker compose up -d url-shortener   # no --build needed

# 3. Run test
k6 run --summary-export=results.json scripts/loadtest/redirect.js

# 4. Profile during test (separate terminal)
curl -s -o cpu.pprof 'http://localhost:6060/debug/pprof/profile?seconds=30'
go tool pprof -http=:8081 cpu.pprof

# 5. Compare two runs
python3 scripts/loadtest/compare.py before.json after.json
```
