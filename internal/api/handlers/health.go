package handlers

import (
	"context"
	"log/slog"
	"net/http"
)

type HealthService interface {
	Check(ctx context.Context) error
}

func NewHealthHandler(
	svc HealthService,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Check(r.Context()); err != nil {
			slog.Error("ping handler failed", slog.Any("error", err))
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
