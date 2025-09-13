package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/fragpit/gophermart/internal/model"
)

type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

func NewAuthRegisterHandler(svc AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var authReq authRequest
		ValidateParseJSONRequest(w, r, &authReq)

		token, err := svc.Register(r.Context(), authReq.Login, authReq.Password)
		if err != nil {
			slog.Error(
				"failed to register user",
				slog.String("user", authReq.Login),
				slog.Any("error", err),
			)
			switch {
			case errors.Is(err, model.ErrUserExists):
				http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
			case errors.Is(err, model.ErrPasswordPolicyViolated):
				http.Error(
					w,
					"password policy violated",
					http.StatusBadRequest,
				)
			default:
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
			return
		}

		authJSONResponse(w, r, token)
	})
}

func NewAuthLoginHandler(svc AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var authReq authRequest
		ValidateParseJSONRequest(w, r, &authReq)

		token, err := svc.Login(r.Context(), authReq.Login, authReq.Password)
		if err != nil {
			slog.Error(
				"failed to register user",
				slog.String("user", authReq.Login),
				slog.Any("error", err),
			)
			switch {
			case errors.Is(err, model.ErrInvalidCredentials):
				http.Error(w, "wrong username or password", http.StatusUnauthorized)
			default:
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
			return
		}

		authJSONResponse(w, r, token)
	})
}

func authJSONResponse(
	w http.ResponseWriter,
	r *http.Request,
	token string,
) {
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

	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", authResp.Token))
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(b); err != nil {
		slog.Warn("failed to write response", slog.Any("error", err))
		return
	}
}
