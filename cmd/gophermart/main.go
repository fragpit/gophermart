package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/fragpit/gophermart/internal/api/router"
	"github.com/fragpit/gophermart/internal/config"
	collector "github.com/fragpit/gophermart/internal/service/accrual-collector"
	"github.com/fragpit/gophermart/internal/service/auth"
	"github.com/fragpit/gophermart/internal/service/balance"
	"github.com/fragpit/gophermart/internal/service/healthcheck"
	"github.com/fragpit/gophermart/internal/service/orders"
	"github.com/fragpit/gophermart/internal/service/withdrawals"
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

	slog.Info("starting app")

	pgStorage, err := postgresql.NewStorage(ctx, cfg.DatabaseURI)
	if err != nil {
		slog.Error("failed to initialize storage", slog.Any("error", err))
		os.Exit(1)
	}

	routerDeps := buildRouterDeps(cfg, pgStorage)
	router := router.NewRouter(routerDeps)

	wg := &sync.WaitGroup{}
	var exitCode int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("starting api", slog.String("address", cfg.RunAddress))
		if err := router.Run(ctx, cfg.RunAddress); err != nil {
			slog.Error("api failed", slog.Any("error", err))
			atomic.StoreInt32(&exitCode, 1)
			cancel()
			return
		}
		slog.Info("api shut down gracefully")
	}()

	collector := collector.NewCollector(
		cfg.AccrualSystemAddress,
		cfg.AccrualPollInterval,
		pgStorage.Collector,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info(
			"starting collector",
			slog.Duration("interval", cfg.AccrualPollInterval),
		)
		if err := collector.Run(ctx); err != nil {
			slog.Error("collector failed", slog.Any("error", err))
			atomic.StoreInt32(&exitCode, 1)
			cancel()
			return
		}
		slog.Info("collector shutdown gracefully")
	}()

	wg.Wait()

	ec := int(atomic.LoadInt32(&exitCode))
	if ec != 0 {
		slog.Error("app failed", slog.Int("exit_code", ec))
		os.Exit(ec)
	}

	slog.Info("app shut down successfully")
}

func buildRouterDeps(
	cfg *config.Config,
	st *postgresql.Repositories,
) router.StorageDeps {
	healthSvc := healthcheck.NewHealthcheckService(st.Health)
	authSvc := auth.NewAuthService(
		st.Users,
		cfg.JWTSecret,
		cfg.JWTTTL,
	)
	ordersSvc := orders.NewOrdersService(st.Orders)
	balanceSvc := balance.NewBalanceService(st.Balance)
	withdrawalsSvc := withdrawals.NewWithdrawalsService(
		st.Withdrawals,
	)
	return router.StorageDeps{
		JWTSecret:          cfg.JWTSecret,
		HealthService:      healthSvc,
		AuthService:        authSvc,
		OrdersService:      ordersSvc,
		BalanceService:     balanceSvc,
		WithdrawalsService: withdrawalsSvc,
	}
}
