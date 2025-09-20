package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fragpit/gophermart/internal/model"
)

//go:generate mockgen -destination ./mocks/balance_mock.go . BalanceService
type BalanceService interface {
	GetUserBalance(ctx context.Context, userID int) (model.Kopek, error)
	GetWithdrawalsSum(ctx context.Context, userID int) (model.Kopek, error)
	WithdrawPoints(
		ctx context.Context,
		userID int,
		orderNum string,
		sum model.Kopek,
	) error
}

type balanceResponse struct {
	CurrentBalance model.Kopek `json:"current"`
	TotalWithdrawn model.Kopek `json:"withdrawn"`
}

func NewBalanceHandler(svc BalanceService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID int
		var ok bool
		ctx := r.Context()
		if userID, ok = UserIDFromContext(ctx); !ok {
			slog.Error(
				"orders request error",
				slog.String("error", "failed to get user id from context"),
			)
			http.Error(
				w,
				http.StatusText(http.StatusUnauthorized),
				http.StatusUnauthorized,
			)
			return
		}

		balance, err := svc.GetUserBalance(ctx, userID)
		if err != nil {
			slog.Error("balance request error", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		withdrawals, err := svc.GetWithdrawalsSum(ctx, userID)
		if err != nil {
			slog.Error("balance request error", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		resp := &balanceResponse{
			CurrentBalance: balance,
			TotalWithdrawn: withdrawals,
		}

		b, err := json.Marshal(resp)
		if err != nil {
			slog.Warn("failed to marshal json response", slog.Any("error", err))
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if _, err := w.Write(b); err != nil {
			slog.Warn("failed to write response", slog.Any("error", err))
			return
		}

	})
}

type balanceWithdrawRequest struct {
	OrderNum string      `json:"order"`
	Sum      model.Kopek `json:"sum"`
}

func NewBalanceWithdrawHandler(svc BalanceService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			slog.Error(
				"request with an empty or unsupported content type",
				slog.String("content_type", r.Header.Get("Content-Type")),
			)
			http.Error(w, "wrong content type", http.StatusUnsupportedMediaType)
			return
		}

		var userID int
		var ok bool
		ctx := r.Context()
		if userID, ok = UserIDFromContext(ctx); !ok {
			slog.Error(
				"orders request error",
				slog.String("error", "failed to get user id from context"),
			)
			http.Error(
				w,
				http.StatusText(http.StatusUnauthorized),
				http.StatusUnauthorized,
			)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer func() { _ = r.Body.Close() }()

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var withdrawRequest balanceWithdrawRequest
		if err := dec.Decode(&withdrawRequest); err != nil {
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

		orderNumber := strings.TrimSpace(string(withdrawRequest.OrderNum))
		if orderNumber == "" {
			slog.Error(
				"failed to read body",
				slog.String("error", "empty order number"),
			)
			http.Error(w, "empty order number", http.StatusBadRequest)
			return
		}

		if !model.ValidateNumber(orderNumber) {
			slog.Error(
				"failed to validate order number",
				slog.String("error", "failed to validate order number"),
			)
			http.Error(
				w,
				"failed to validate order number",
				http.StatusUnprocessableEntity,
			)
			return
		}

		if !model.ValidateSum(withdrawRequest.Sum) {
			slog.Error(
				"failed to validate sum",
				slog.String("error", "failed to validate sum"),
			)
			http.Error(
				w,
				"failed to validate sum",
				http.StatusUnprocessableEntity,
			)
			return
		}

		if err := svc.WithdrawPoints(
			ctx,
			userID,
			withdrawRequest.OrderNum,
			withdrawRequest.Sum,
		); err != nil {
			slog.Warn("error withdrawing points", slog.Any("error", err))
			switch {
			case errors.Is(err, model.ErrInsufficientPoints):
				http.Error(
					w,
					"insufficient points",
					http.StatusPaymentRequired,
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

		w.WriteHeader(http.StatusOK)
	})
}
