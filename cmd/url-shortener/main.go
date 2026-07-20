package main

import (
	"context"
	"github.com/DimaOshchepkov/url_shortener/internal/app"
	"github.com/DimaOshchepkov/url_shortener/internal/config"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogpretty"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/sl"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

//	@title			URL Shortener API
//	@version		1.0
//	@description	REST API сервиса сокращения ссылок с JWT-аутентификацией.
//
//	@host		localhost:8080
//	@BasePath	/

// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				JWT токен в формате "Bearer &lt;token&gt;". Получить токен через SSO-сервис.

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)
	log.Info("starting url shortener", slog.String("env", cfg.Env))
	log.Debug("creddentials url-shortener", slog.String("address", cfg.Address))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.RunServer(ctx, log, cfg); err != nil {
		log.Error("error to start server", sl.Err(err))
	} else {
		log.Info("server was shutdown")
	}
}

func setupLogger(env string) *slog.Logger {
	switch env {
	case envLocal:
		return setupPrettySlog()
	case envDev:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := opts.NewPrettyHandler(os.Stdout)
	return slog.New(handler)
}
