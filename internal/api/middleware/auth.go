package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fragpit/gophermart/internal/service/auth"
)

type ctxKey string

const СtxUserIDKey ctxKey = "user_id"

func RequireJWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if authz == "" {
				slog.Error(
					"authentication error",
					slog.String("error", "header not set"),
				)
				http.Error(
					w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			parts := strings.SplitN(authz, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
				parts[1] == "" {
				http.Error(
					w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			token := parts[1]
			userID, err := auth.GetUserIDFromJWTToken(secret, token)
			if err != nil {
				slog.Warn("invalid jwt", slog.Any("error", err))
				http.Error(
					w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			ctx := context.WithValue(r.Context(), СtxUserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
