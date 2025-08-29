package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fragpit/gophermart/internal/api/router"
	"github.com/fragpit/gophermart/internal/auth"
	"github.com/fragpit/gophermart/internal/config"
	"github.com/fragpit/gophermart/internal/healthcheck"
	"github.com/fragpit/gophermart/internal/storage/postgresql"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancel()

	cfg, err := config.NewConfig()
	if err != nil {
		slog.Error("failed to initialize config", slog.Any("error", err))
		fmt.Println(cfg)
		os.Exit(1)
	}

	var logLevel slog.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case slog.LevelDebug.String():
		logLevel = slog.LevelDebug
	case slog.LevelWarn.String():
		logLevel = slog.LevelWarn
	case slog.LevelError.String():
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	if cfg.LogLevel == "debug" {
		slog.Debug("running with config")
		fmt.Println(cfg.String())
	}

	logger.Info("starting server", slog.String("address", cfg.RunAddress))

	pgStorage, err := postgresql.NewStorage(ctx, cfg.DatabaseURI)
	if err != nil {
		slog.Error("failed to initialize storage", slog.Any("error", err))
		os.Exit(1)
	}

	authSvc := auth.NewAuthService(pgStorage, cfg.JWTSecret, cfg.JWTTTL)
	healthSvc := healthcheck.NewHealthcheckService(pgStorage)

	routerDeps := router.StorageDeps{
		HealthService: healthSvc,
		AuthService:   authSvc,
		JWTSecret:     cfg.JWTSecret,
	}
	router := router.NewRouter(routerDeps)
	if err := router.Run(ctx, cfg.RunAddress); err != nil {
		logger.Error("failed to start api", slog.Any("error", err))
		logger.Info("shutting down app")
		cancel()
	}

	logger.Info("server shut down")
}
