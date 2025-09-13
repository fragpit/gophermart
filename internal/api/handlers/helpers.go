package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/fragpit/gophermart/internal/api/middleware"
)

func UserIDFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(middleware.CtxUserIDKey)
	if v == nil {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

func ValidateParseJSONRequest(
	w http.ResponseWriter,
	r *http.Request,
	data any,
) {
	// validate header
	if r.Header.Get("Content-Type") != "application/json" {
		slog.Error(
			"request with an empty or unsupported content type",
			slog.String("content_type", r.Header.Get("Content-Type")),
		)
		http.Error(w, "wrong content type", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() { _ = r.Body.Close() }()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&data); err != nil {
		var mberr *http.MaxBytesError
		slog.Warn("invalid JSON", slog.Any("error", err))
		if errors.As(err, &mberr) {
			http.Error(
				w,
				"request body too large",
				http.StatusRequestEntityTooLarge,
			)
			return
		}
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := dec.Decode(&struct{}{}); err != io.EOF {
		slog.Warn("invalid JSON", slog.Any("error", err))
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
}
