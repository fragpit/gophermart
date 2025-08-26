package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) error
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

// `POST /api/user/register` — регистрация пользователя;
func NewAuthRegisterHandler(svc AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		var authReq authRequest
		if err := dec.Decode(&authReq); err != nil {
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

		token, err := svc.Register(r.Context(), authReq.Login, authReq.Password)
		if err != nil {
			slog.Warn("failed to register user", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		authResp := &authResponse{
			Token: token,
		}

		b, err := json.Marshal(authResp)
		if err != nil {
			slog.Warn("failed to marshal json response", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(b); err != nil {
			slog.Warn("failed to write response", slog.Any("error", err))
			return
		}
	})
}

// `POST /api/user/login` — аутентификация пользователя;
func NewAuthLoginHandler(svc AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}
