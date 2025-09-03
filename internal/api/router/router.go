package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fragpit/gophermart/internal/api/handlers"
	"github.com/fragpit/gophermart/internal/api/middleware" // Keeping middleware import
)

const apiShutdownTimeout = 5 * time.Second

type StorageDeps struct {
	JWTSecret string

	HealthService      handlers.HealthService
	AuthService        handlers.AuthService
	OrdersService      handlers.OrdersService
	BalanceService     handlers.BalanceService
	WithdrawalsService handlers.WithdrawalsService
}

type Router struct {
	router http.Handler
}

func NewRouter(deps StorageDeps) *Router {
	mux := http.NewServeMux()

	authMW := middleware.RequireJWT(deps.JWTSecret)

	mux.Handle(
		"GET /health",
		handlers.NewHealthHandler(deps.HealthService),
	)

	mux.Handle(
		"POST /api/user/register",
		handlers.NewAuthRegisterHandler(deps.AuthService),
	)
	mux.Handle(
		"POST /api/user/login",
		handlers.NewAuthLoginHandler(deps.AuthService),
	)

	mux.Handle(
		"GET /api/user/orders",
		authMW(handlers.NewOrdersGetHandler(deps.OrdersService)),
	)
	mux.Handle(
		"POST /api/user/orders",
		authMW(handlers.NewOrdersPostHandler(deps.OrdersService)),
	)

	mux.Handle(
		"GET /api/user/balance",
		authMW(handlers.NewBalanceHandler(deps.BalanceService)),
	)
	mux.Handle(
		"POST /api/user/balance/withdraw",
		authMW(handlers.NewBalanceWithdrawHandler(deps.BalanceService)),
	)

	mux.Handle(
		"GET /api/user/withdrawals",
		authMW(handlers.NewWithdrawalsHandler(deps.WithdrawalsService)),
	)

	return &Router{
		router: mux,
	}
}

func (r *Router) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: r.router,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", slog.Any("error", err))
			errChan <- err
			return
		}
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(ctx, apiShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error(
				"failed to shutdown server gracefully",
				slog.Any("error", err),
			)
			return err
		}

		slog.Info("api shut down gracefully")
	}

	return nil
}
