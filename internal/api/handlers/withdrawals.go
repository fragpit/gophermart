package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/fragpit/gophermart/internal/model"
)

type WithdrawalsService interface {
	GetWithdrawalsByUser(
		ctx context.Context,
		userID int,
	) ([]model.Withdrawal, error)
}

type WithdrawalsResponse struct {
	OrderNumber  string      `json:"order"`
	SumWithdrawn model.Kopek `json:"sum"`
	ProcessedAt  string      `json:"processed_at"`
}

func NewWithdrawalsHandler(svc WithdrawalsService) http.Handler {
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

		withdrawals, err := svc.GetWithdrawalsByUser(ctx, userID)
		if err != nil {
			slog.Error(
				"orders request error",
				slog.Any("error", err),
			)
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if len(withdrawals) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var response []WithdrawalsResponse
		for _, wd := range withdrawals {
			r := WithdrawalsResponse{
				OrderNumber:  wd.OrderNum,
				SumWithdrawn: wd.Sum,
				ProcessedAt:  wd.ProcessedAt.Format(time.RFC3339),
			}
			response = append(response, r)
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("encode orders error", slog.Any("error", err))
		}
	})
}
