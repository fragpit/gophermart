package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fragpit/gophermart/internal/model"
)

type OrdersService interface {
	GetOrdersByUser(ctx context.Context, userID int) ([]model.Order, error)
	AddOrder(
		ctx context.Context,
		userID int,
		orderNumber string,
	) error
}

type ordersGetResponse struct {
	Number     string            `json:"number"`
	Status     model.OrderStatus `json:"status"`
	Accrual    model.Kopek       `json:"accrual"`
	UploadedAt string            `json:"uploaded_at"`
}

func NewOrdersGetHandler(svc OrdersService) http.Handler {
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

		orders, err := svc.GetOrdersByUser(ctx, userID)
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
		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var response []ordersGetResponse
		for _, order := range orders {
			r := ordersGetResponse{
				Number:     order.Number,
				Status:     order.Status,
				Accrual:    order.Accrual,
				UploadedAt: order.UploadedAt.Format(time.RFC3339),
			}
			response = append(response, r)
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("encode orders error", slog.Any("error", err))
		}
	})
}

func NewOrdersPostHandler(svc OrdersService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/plain" {
			slog.Error(
				"request with an empty or unsupported content type",
				slog.String("content_type", r.Header.Get("Content-Type")),
			)
			http.Error(w, "wrong content type", http.StatusUnsupportedMediaType)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read body", slog.Any("error", err))
			http.Error(w, "invalid order number", http.StatusBadRequest)
			return
		}
		orderNumber := strings.TrimSpace(string(body))
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

		if err := svc.AddOrder(ctx, userID, orderNumber); err != nil {
			if errors.Is(err, model.ErrOrderAlreadyExist) {
				slog.Info("order already added")
				http.Error(w, "order already added", http.StatusOK)
			} else if errors.Is(err, model.ErrOrderAlreadyAddedByOtherUser) {
				slog.Info("order already added by other user")
				http.Error(w, "order already added by other user", http.StatusConflict)
			}

			return
		}

		w.WriteHeader(http.StatusAccepted)
	})
}
