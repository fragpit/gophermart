package handlers

import (
	"context"
	"log/slog"
	"net/http"
)

//go:generate mockgen -destination ./mocks/health_mock.go . HealthService
type HealthService interface {
	Check(ctx context.Context) error
}

func NewHealthHandler(
	svc HealthService,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Check(r.Context()); err != nil {
			slog.Error("health check failed", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
