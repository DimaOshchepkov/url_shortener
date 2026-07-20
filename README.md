<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/neepooha/url_shortener/raw/main/assets/images/logo-white.png">
    <img alt="logo" src="https://github.com/neepooha/url_shortener/raw/main/assets/images/logo-black.png" width="40%">
  </picture>
</div>

<br><br>

<div align="center">
  
![License](https://img.shields.io/badge/License-MIT-red)
![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
[![Go Report Card](https://goreportcard.com/badge/github.com/neepooha/url_shortener)](https://goreportcard.com/report/github.com/neepooha/url_shortener)
[![Go Reference](https://pkg.go.dev/badge/github.com/neepooha/url_shortener.svg)](https://pkg.go.dev/github.com/neepooha/url_shortener)
[![Build Status](https://github.com/neepooha/url_shortener/actions/workflows/deploy.yml/badge.svg)](https://github.com/neepooha/url_shortener/actions/workflows/deploy.yml)
<br>
</div>

## Features
The template for this project was taken from the project of [Nikolay Tuzov](https://github.com/JustSkiv).
This project was implemented for the purpose of learning, knowledge [SOLID principles](https://en.wikipedia.org/wiki/SOLID) 
and [clean architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).
It encourages writing clean and idiomatic Go code.


### Technical Highlights

#### Performance

- **LRU cache with TTL** — [`hashicorp/golang-lru/v2`](https://github.com/hashicorp/golang-lru) with configurable max size and TTL. Cache-aside pattern: hot aliases served from memory, **p50 dropped 66%** (36ms → 12ms) with 99.9% hit rate at 4,200 RPS.
- **ClickBatcher** — async write coalescing. Click increments are buffered in-memory and flushed as batched `UPDATE` statements, reducing DB writes from one-per-request to one-per-alias-per-flush-interval.
- **Connection pooling** — [`pgxpool`](https://github.com/jackc/pgx) with configurable `MinConns`/`MaxConns` for predictable database load.
- **pprof-driven optimization** — CPU profiling revealed structured logging consumed 25% of CPU on the redirect hot path. After removing the logger middleware from `GET /{alias}`, overall p50 improved **75.5%** (36ms → 8.9ms) and throughput reached **4,542 RPS**.

#### Observability

- **Prometheus metrics** — 4 metrics: redirect requests (by status), latency histogram, cache hits/misses. Per-alias labels deliberately avoided to prevent cardinality explosion — per-link analytics live in PostgreSQL.
- **Grafana dashboards** — auto-provisioned via `grafana/`. 5 panels out of the box: redirect rate, latency (p50/p90/p99), cache hit rate, errors by status, and top aliases by clicks (direct PostgreSQL query).
- **Structured logging** — [`log/slog`](https://pkg.go.dev/log/slog) with pretty handler for local development and JSON handler for production.
- **Health check** — `GET /health` returns cache statistics (hit rate, entry count).
- **pprof** — `:6060/debug/pprof/` on a separate port, not exposed to the public API.

#### Security

- **JWT hardening** — HMAC signing method explicitly enforced in the auth middleware, preventing `alg: none` attacks. Claims are type-validated (`uid`, `app_id`).
- **Open redirect protection** — URL scheme validation before 302 redirect rejects `javascript:`, `ftp://`, and other non-HTTP schemes.
- **Slowloris protection** — `ReadHeaderTimeout: 2s` limits header read duration.
- **RBAC** — admin endpoints verify permissions via gRPC call to the SSO service.

#### Architecture

- **Clean Architecture** — handlers → storage interface → PostgreSQL, following [SOLID principles](https://en.wikipedia.org/wiki/SOLID) and dependency inversion.
- **Decorator chain** — `PostgreSQL → ClickBatcher → Cache → HTTP handler`. Each layer wraps the next without the layers above knowing about the layers below.
- **Facade pattern** — a common interface (`GetURL` + `IncrementClicks` + `HitRate` + `Len`) lets handlers work with either a real cache or a pass-through stub — zero handler changes when toggling caching.
- **Typed context keys** — middleware values (`uid`, `app_id`) stored with typed keys to prevent collisions.

#### CI/CD & DevOps

- **GitHub Actions** — CI on every push: `go test -race`, `go vet`, [`golangci-lint`](https://github.com/golangci/golangci-lint), and build. Manual deploy workflow (`workflow_dispatch`) builds the binary, syncs via `rsync`, writes `config.env` from GitHub Secrets, and restarts the systemd service.
- **Single config file** — `config/config.yaml` with local defaults, overridden via environment variables for Docker/production (similar to Laravel's `.env` approach). Zero duplication across environments.
- **Docker Compose** — 4 services: app, PostgreSQL, Prometheus, Grafana. External `auth-network` for SSO inter-service communication.
- **systemd** — production service with `Restart=always`, `EnvironmentFile`, and `WantedBy=multi-user.target`.
- **Database migrations** — [`golang-migrate`](https://github.com/golang-migrate/migrate) run automatically on startup.

#### API & Documentation

- **RESTful API** on [`go-chi`](https://github.com/go-chi/chi) — lightweight, idiomatic router with middleware chaining.
- **Swagger/OpenAPI** — auto-generated via [`swaggo/swag`](https://github.com/swaggo/swag), served at `/swagger/index.html`. All 6 endpoints annotated.
- **gRPC client** — SSO integration for authentication and admin permission checks.
- **k6 load tests** — realistic Zipf-distribution traffic (80% hot / 20% cold aliases) with SLO thresholds.

### The kit uses the following Go packages:

* Routing: [go-chi](https://github.com/go-chi/chi)
* Database access: [pgx](https://github.com/jackc/pgx)
* Database migration: [golang-migrate](https://github.com/golang-migrate/migrate)
* Data validation: [go-playground validator](https://github.com/go-playground/validator)
* Logging: [log/slog](https://pkg.go.dev/golang.org/x/exp/slog)
* JWT: [jwt-go](https://github.com/dgrijalva/jwt-go)
* Config reader: [cleanenv](https://github.com/ilyakaznacheev/cleanenv)
* Env reader: [godotenv](https://github.com/joho/godotenv)
* Prometheus metrics: [prometheus/client_golang](https://github.com/prometheus/client_golang)
* Swagger docs: [swaggo/swag](https://github.com/swaggo/swag)
* Cache: [hashicorp/golang-lru](https://github.com/hashicorp/golang-lru)
<br>


<div align="center">
  scheme of user interaction, SSO and URL-shortener
  <br>
  <picture>
    <img alt="scheme" src="https://github.com/neepooha/url_shortener/raw/main/assets/images/scheme.png" width="60%">
  </picture>
</div>

> [!IMPORTANT]\
> This project is a microservice that works in conjunction with a SSO. For full functionality, you need to have two microservices running.<br>
[![SSO](https://github-readme-stats.vercel.app/api/pin/?username=neepooha&repo=sso&border_color=7F3FBF&bg_color=0D1117&title_color=C9D1D9&text_color=8B949E&icon_color=7F3FBF)](https://github.com/neepooha/sso)
<br>


## Getting Started

If this is your first time encountering Go, please follow [the instructions](https://golang.org/doc/install) to install Go on your computer. 
The project requires Go 1.25 or above.

[Docker](https://www.docker.com/get-started) is also needed if you want to try the kit without setting up your own database server.
The project requires Docker 17.05 or higher for the multi-stage build support.

Also for simple run commands i use [Taskfile](https://taskfile.dev/installation/). 

After installing Go, Docker and TaskFile, run the following commands to start experiencing:
```shell
## RUN URL-SHORTENER
# download the project
git clone https://github.com/neepooha/url_shortener.git
cd url_shortener

# create config.env with that text:
$ nano config.env {
CONFIG_PATH=./config/config.yaml
POSTGRES_DB=url
POSTGRES_USER=myuser
POSTGRES_PASSWORD=mypass
}

# start a PostgreSQL database server in a Docker container
task db-start

# run the RESTful API server
go run ./cmd/url-shortener

## RUN SSO
# download the project
git clone https://github.com/neepooha/sso.git
cd sso

# create config.env with that text:
$ nano config.env {
CONFIG_PATH=./config/config.yaml
POSTGRES_DB=url
POSTGRES_USER=myuser
POSTGRES_PASSWORD=mypass
}

# start a PostgreSQL database server in a Docker container
task db-start

# run the SSO server
go run ./cmd/sso
```
Also, you can start project in dev mode with Docker Compose. For that add
Docker hostnames to config.env and run:
```shell
# Add Docker host overrides to config.env:
# ENV=dev
# STORAGE_HOST=urldb
# STORAGE_PORT=5432
# HTTP_SERVER_ADDRESS=0.0.0.0:8080
# SSO_ADDRESS=sso:44044

# create a shared Docker network for inter-service communication (once)
docker network create auth-network

# run the RESTful API server with docker-compose
cd url_shortener/
docker compose up --build

# run the SSO server
cd sso/
docker compose up --build
```

At this time, you have a RESTful API server running at http://localhost:8080 and SSO-grpc Server running at http://localhost:44044.  Restful-API server provides the following endpoints:

* `POST /url`: shortens the link using an alias, or if the alias is not specified, then using a random 6-character alias. Requires authentication
* `DELETE /url/{alias}`: remove link by alias. Requires admin privileges
* `GET /{alias}`: redirect by alias (all users)
* `GET /health`: health check with cache stats

* `POST /user`: grant admin permissions. Requires SSO token in Authorization header
* `DELETE /user`: revoke admin permissions. Requires SSO token in Authorization header

**Observability:**
* `:6060/metrics` — Prometheus metrics endpoint
* `:6060/debug/pprof/` — Go pprof profiling
* `http://localhost:3000` — Grafana dashboards (when running with docker-compose)
* `http://localhost:8080/swagger/index.html` — Swagger UI

## Project Layout
Project has the following project layout:
```
url-shortener/
├── cmd/                       start of applications of the project
├── config/                    configuration files for different environments
├── deployment/                configuration for create daemon in linux
├── docs/                      generated Swagger documentation
├── grafana/                   Grafana provisioning (datasources + dashboards)
├── internal/                  private application and library code
│   ├── app/                   application assembly
│   ├── clients/               gRPC servers (only SSO) assembly
│   ├── config/                configuration library
│   ├── lib/                   additional functions
│   │   ├── api/               HTTP helpers (response formatting, redirect)
│   │   ├── clickbatcher/      async click increment batching
│   │   ├── logger/            slog handlers (pretty, discard)
│   │   ├── metrics/           Prometheus metric definitions
│   │   ├── migrator/          database migration runner
│   │   └── random/            crypto-random string generator
│   ├── storage/               storage library
│   │   ├── cache/             in-memory LRU cache + pass-through
│   │   └── postgres/          PostgreSQL storage (pgxpool)
│   └── transport/             handlers and middlewares
│       ├── handlers/          handlers
│       │   ├── admins/        handlers to set/delete admins
│       │   ├── health/        health check endpoint
│       │   └── url/           handlers for URL CRUD + redirect
│       └── middleware/        middlewares
│           ├── auth/          JWT authentication
│           ├── context/       typed context keys
│           ├── isadmin/       admin permission check
│           └── logger/        request logging
├── migrations/                database migrations
├── scripts/loadtest/          k6 load testing scripts
├── .github/workflows/         CI/CD pipelines
├── .gitignore
├── config.env                 environment variables
└── prometheus.yml             Prometheus scrape config
```
The top level directories `cmd`, `internal`, `lib` are commonly found in other popular Go projects, as explained in
[Standard Go Project Layout](https://github.com/golang-standards/project-layout).

Within `internal` package are structured by features in order to achieve the so-called
[screaming architecture](https://blog.cleancoder.com/uncle-bob/2011/09/30/Screaming-Architecture.html). For example, 
the `transport` directory contains the application logic related with the entity feature. 

Within each feature package, code are organized in layers (API, service, repository), following the dependency guidelines
as described in the [clean architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

## Updating Database Schema
for simple migration you can use the following commands
```shell
# For up migrations
task up

# For drop migrations
task drop

# Revert the last database migration.
task rollback
```
## Testing

```shell
# Run all tests with race detector
go test -race ./...

# Run tests for a specific package
go test -race ./internal/storage/cache/

# Generate mocks (requires mockery)
go generate ./...

# Generate Swagger docs (requires swag CLI)
task swagger
```

## Swagger Documentation

API documentation is auto-generated from source code annotations using [`swaggo/swag`](https://github.com/swaggo/swag) and served at `/swagger/index.html`.

**Generation:**
```shell
# Generate docs from handler annotations
task swagger
```
This runs `swag init -g cmd/url-shortener/main.go -o docs/` and produces three files in `docs/`:
- `docs.go` — Go source embedding the full spec (auto-registered on import)
- `swagger.json` — OpenAPI 2.0 spec in JSON
- `swagger.yaml` — OpenAPI 2.0 spec in YAML

**Documented endpoints:**

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| `GET` | `/health` | — | Service health + cache stats |
| `POST` | `/url` | Bearer JWT | Create short link (optional alias, random 6-char if omitted) |
| `DELETE` | `/url/{alias}` | Bearer JWT + admin | Delete alias |
| `GET` | `/{alias}` | — | Redirect to original URL |
| `POST` | `/user` | SSO token | Grant admin privileges |
| `DELETE` | `/user` | SSO token | Revoke admin privileges |

Annotations live in the handler source files (`internal/transport/handlers/`). The Swagger UI is mounted in `internal/app/app.go` via `httpSwagger.WrapHandler`.

## Performance Tuning

Three systematic optimization rounds, guided by pprof profiling and k6 load testing, reduced median redirect latency by **75.5%** and improved throughput by **18.2%**.

### Round 1: Baseline (no cache)

Direct PostgreSQL reads per request. **3,843 req/s**, p50 **36.4ms**, p95 **220ms**. Bottleneck: database.

### Round 2: In-Memory LRU Cache

Added cache-aside layer with 10K entries and 5min TTL. **99.9% hit rate**, p50 dropped **66%** to **12.3ms**. Throughput: **4,204 req/s** (+9.4%).

### Round 3: Remove Logging from Redirect Path

pprof captured during Round 2 revealed `slog` JSON marshaling consumed **25% of CPU** on the redirect hot path. After removing the logger middleware from `GET /{alias}`, p50 dropped another **27%** to **8.9ms**. Throughput: **4,542 req/s** (+8.0%).

### Full Comparison

| Metric | R1 (No Cache) | R2 (+Cache) | R3 (−Log) | **Total Improvement** |
|--------|:---:|:---:|:---:|:---:|
| Throughput | 3,843/s | 4,204/s | 4,542/s | **+18.2%** |
| p50 | 36.37ms | 12.26ms | 8.90ms | **−75.5%** |
| p95 | 220.40ms | 209.22ms | 164.33ms | **−25.4%** |
| p99 | 325.02ms | 322.05ms | 256.01ms | **−21.2%** |
| Avg | 67.80ms | 51.45ms | 38.89ms | **−42.6%** |

> Full load test protocol, k6 scenarios, Zipf distribution setup, and pprof analysis commands are documented in [`scripts/loadtest/README.md`](scripts/loadtest/README.md).

## Managing Configurations

The project uses a single `config/config.yaml` with local defaults. Environment-specific values are overridden
via environment variables (through `config.env`). See `internal/config/config.go` for all available fields.

**Local development** (`go run`):
```shell
# config.env
CONFIG_PATH=./config/config.yaml
POSTGRES_DB=url
POSTGRES_USER=myuser
POSTGRES_PASSWORD=mypass
```

**Docker/dev** (`docker compose`):
```shell
# config.env — Docker host overrides
CONFIG_PATH=./config/config.yaml
ENV=dev
STORAGE_HOST=urldb
STORAGE_PORT=5432
HTTP_SERVER_ADDRESS=0.0.0.0:8080
SSO_ADDRESS=sso:44044
POSTGRES_DB=url
POSTGRES_USER=myuser
POSTGRES_PASSWORD=mypass
```

**Production** — `config.env` is generated automatically by GitHub Actions (`deploy.yml`).
Secrets (`APP_SECRET`, `HTTP_SERVER_PASSWORD`, `POSTGRES_*`) are stored in GitHub Secrets.
