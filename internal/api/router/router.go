package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const apiShutdownTimeout = 5 * time.Second

type Router struct {
	logger *slog.Logger
	router http.Handler
}

func NewRouter(logger *slog.Logger) *Router {
	r := &Router{
		logger: logger,
	}
	r.router = r.initRoutes() // TODO: do I need it?

	return r
}

// TODO: do I need it?
func (r *Router) initRoutes() http.Handler {
	return nil
}

func (r *Router) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: r.router,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.logger.Error("failed to start server", slog.Any("error", err))
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
			r.logger.Error(
				"failed to shutdown server gracefully",
				slog.Any("error", err),
			)
			return err
		}

		r.logger.Info("api shut down gracefully")
	}

	return nil
}
