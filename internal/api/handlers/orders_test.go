package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mock_handlers "github.com/fragpit/gophermart/internal/api/handlers/mocks"
	"github.com/fragpit/gophermart/internal/api/middleware"
	"github.com/fragpit/gophermart/internal/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const (
	orderNumByLuhn = "79927398713"
)

func TestOrdersGetHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type mockData struct {
		orders []model.Order
		err    error
	}

	tests := []struct {
		name       string
		mockData   mockData
		authUserID int
		wantCode   int
	}{
		{
			name: "success",
			mockData: mockData{
				orders: []model.Order{
					{
						ID:         1,
						UserID:     1,
						Number:     orderNumByLuhn,
						Status:     model.StatusNew,
						Accrual:    50,
						UploadedAt: time.Now(),
					},
				},
				err: nil,
			},
			authUserID: 1,
			wantCode:   http.StatusOK,
		},
		{
			name: "success no data",
			mockData: mockData{
				orders: []model.Order{},
				err:    nil,
			},
			authUserID: 1,
			wantCode:   http.StatusNoContent,
		},
		{
			name:       "fail unauthenticated",
			mockData:   mockData{},
			authUserID: 0,
			wantCode:   http.StatusUnauthorized,
		},
		{
			name: "fail internal",
			mockData: mockData{
				orders: nil,
				err:    errors.New("db error"),
			},
			authUserID: 1,
			wantCode:   http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockOrdersService(ctrl)

			m.EXPECT().
				GetOrdersByUser(gomock.Any(), gomock.Any()).
				Return(tc.mockData.orders, tc.mockData.err).AnyTimes()
			handler := NewOrdersGetHandler(m)
			rec := httptest.NewRecorder()

			var ctx context.Context
			if tc.authUserID != 0 {
				ctx = context.WithValue(
					t.Context(),
					middleware.CtxUserIDKey,
					tc.authUserID,
				)
			} else {
				ctx = context.Background()
			}

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)

			if tc.mockData.orders != nil {
				var response []ordersGetResponse
				if err := json.NewDecoder(rec.Body).Decode(&response); err != io.EOF {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestOrdersPostHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type mockData struct {
		err error
	}

	tests := []struct {
		name        string
		mockData    mockData
		reqBody     *ordersGetResponse
		orderNumber string
		authUserID  int
		wantCode    int
	}{
		{
			name: "success",
			mockData: mockData{
				err: nil,
			},
			orderNumber: orderNumByLuhn,
			authUserID:  1,
			wantCode:    http.StatusAccepted,
		},
		{
			name: "success order already added",
			mockData: mockData{
				err: model.ErrOrderAlreadyExist,
			},
			orderNumber: orderNumByLuhn,
			authUserID:  1,
			wantCode:    http.StatusOK,
		},
		{
			name: "error order already added by other user",
			mockData: mockData{
				err: model.ErrOrderAlreadyAddedByOtherUser,
			},
			orderNumber: orderNumByLuhn,
			authUserID:  1,
			wantCode:    http.StatusConflict,
		},
		{
			name:        "error empty order number",
			mockData:    mockData{},
			orderNumber: "",
			authUserID:  1,
			wantCode:    http.StatusBadRequest,
		},
		{
			name:        "error invalid order number",
			mockData:    mockData{},
			orderNumber: "123123",
			authUserID:  1,
			wantCode:    http.StatusUnprocessableEntity,
		},
		{
			name:        "fail unauthenticated",
			mockData:    mockData{},
			orderNumber: orderNumByLuhn,
			authUserID:  0,
			wantCode:    http.StatusUnauthorized,
		},
		{
			name: "fail internal",
			mockData: mockData{
				err: errors.New("db error"),
			},
			orderNumber: orderNumByLuhn,
			authUserID:  1,
			wantCode:    http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockOrdersService(ctrl)

			m.EXPECT().
				AddOrder(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tc.mockData.err).AnyTimes()
			handler := NewOrdersPostHandler(m)
			rec := httptest.NewRecorder()

			var ctx context.Context
			if tc.authUserID != 0 {
				ctx = context.WithValue(
					t.Context(),
					middleware.CtxUserIDKey,
					tc.authUserID,
				)
			} else {
				ctx = context.Background()
			}

			data := strings.NewReader(tc.orderNumber)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/", data)
			req.Header.Set("Content-Type", "text/plain")

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
		})
	}
}
