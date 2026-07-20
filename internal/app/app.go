package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	ssogrpc "github.com/DimaOshchepkov/url_shortener/internal/clients/sso/grpc"
	"github.com/DimaOshchepkov/url_shortener/internal/config"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/clickbatcher"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/sl"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/migrator"
	"github.com/DimaOshchepkov/url_shortener/internal/storage/cache"
	"github.com/DimaOshchepkov/url_shortener/internal/storage/postgres"
	admDel "github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/admins/delete"
	admSet "github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/admins/set"
	"github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/health"
	urlDel "github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/url/delete"
	urlRed "github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/url/redirect"
	urlSave "github.com/DimaOshchepkov/url_shortener/internal/transport/handlers/url/save"
	"github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/auth"
	"github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/isadmin"
	mwLogger "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/DimaOshchepkov/url_shortener/docs"
)

func RunServer(ctx context.Context, log *slog.Logger, cfg *config.Config) error {
	const op = "internal.app.RunServer"
	log = log.With(slog.String("op", op))

	// init ssoServer
	log.Info("init ssoClinet", slog.String("env", cfg.Env))
	log.Debug("creddentials sso", slog.String("address", cfg.Clients.SSO.Address))
	ssoClient, err := ssogrpc.New(
		context.Background(),
		log, cfg.Clients.SSO.Address,
		cfg.Clients.SSO.Timeout,
		cfg.Clients.SSO.RetriesCount,
	)
	if err != nil {
		log.Error("failed to init ssoClient", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	log.Info("ssoClient was init")

	// init postgresql storage
	storage, err := postgres.NewStorage(cfg)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	defer storage.CloseStorage()

	// start migration
	err = migrator.Migrate(cfg)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Debug("no migrations to apply")
		} else {
			panic(err)
		}
	}
	log.Debug("migrations applied successfully")

	// wrap storage in ClickBatcher to batch click increments in memory
	// (avoids one DB write per redirect — flushes every N seconds instead)
	batcher := clickbatcher.New(storage, log, cfg.ClickBatch.Interval)
	go batcher.Run(ctx)

	// wrap storage with cache for redirects if enabled
	var facade interface {
		GetURL(ctx context.Context, alias string) (string, error)
		IncrementClicks(ctx context.Context, alias string) error
		HitRate() float64
		Len() int
	}
	if cfg.Cache.Enabled {
		cs := cache.New(log, batcher, cfg.Cache.MaxSize, cfg.Cache.TTL)
		facade = cs
		log.Info("cache initialized", slog.Int("max_size", cfg.Cache.MaxSize), slog.Duration("ttl", cfg.Cache.TTL))
	} else {
		facade = cache.NewPassThrough(batcher)
		log.Info("cache disabled")
	}

	// init router
	// Note: mwLogger is NOT applied globally — the redirect endpoint ({alias})
	// skips logging because pprof showed slog JSONHandler consuming 25% CPU.
	// For high-traffic endpoints, k6 metrics and /health provide sufficient observability.
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	// url router
	router.Route("/url", func(r chi.Router) {
		r.Use(mwLogger.New(log))
		r.Use(auth.New(log, cfg.AppSecret))
		r.Post("/", urlSave.New(log, storage))
	})
	router.Route("/url/{alias}", func(r chi.Router) {
		r.Use(mwLogger.New(log))
		r.Use(auth.New(log, cfg.AppSecret))
		r.Use(isadmin.New(log, ssoClient))
		r.Delete("/", urlDel.New(log, storage))
	})
	router.Get("/{alias}", urlRed.New(log, facade))

	// health check
	router.Get("/health", health.New(log, facade))

	// swagger
	router.Get("/swagger/*", httpSwagger.WrapHandler)

	// user router
	router.Route("/user", func(r chi.Router) {
		r.Use(mwLogger.New(log))
		r.Post("/", admSet.New(log, ssoClient))
		r.Delete("/", admDel.New(log, ssoClient))
	})

	// metrics and pprof for profiling (separate observability port, not exposed to public)
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Info("metrics and pprof listening", slog.String("addr", ":6060"), slog.String("metrics", "/metrics"))
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Error("metrics/pprof server failed", sl.Err(err))
		}
	}()

	// start server
	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadTimeout:       cfg.HTTPServer.Timeout,
		WriteTimeout:      cfg.HTTPServer.Timeout,
		IdleTimeout:       cfg.HTTPServer.IdleTimeout,
		ReadHeaderTimeout: cfg.HTTPServer.ReadHeaderTimeout,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("failed to start server")
			os.Exit(1)
		}
	}()
	log.Info("url shortener is running", slog.String("addresses", srv.Addr))

	// wait for gracefully shutdown
	<-ctx.Done()
	log.Info("shutting down server gracefully")
	shutDownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutDownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	<-shutDownCtx.Done()
	return nil
}
