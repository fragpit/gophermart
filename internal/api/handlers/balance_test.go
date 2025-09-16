package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_handlers "github.com/fragpit/gophermart/internal/api/handlers/mocks"
	"github.com/fragpit/gophermart/internal/api/middleware"
	"github.com/fragpit/gophermart/internal/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestBalanceHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type mockData struct {
		sumBalance model.Kopek
		sumWD      model.Kopek
		err        error
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
				sumBalance: 1,
				sumWD:      1,
				err:        nil,
			},
			authUserID: 1,
			wantCode:   http.StatusOK,
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
				err: errors.New("db error"),
			},
			authUserID: 1,
			wantCode:   http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockBalanceService(ctrl)
			m.EXPECT().
				GetUserBalance(gomock.Any(), gomock.Any()).
				Return(tc.mockData.sumBalance, tc.mockData.err).
				AnyTimes()
			m.EXPECT().
				GetWithdrawalsSum(gomock.Any(), gomock.Any()).
				Return(tc.mockData.sumWD, tc.mockData.err).
				AnyTimes()

			handler := NewBalanceHandler(m)
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
		})
	}
}

func TestBalanceWithdrawHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type mockData struct {
		err error
	}

	tests := []struct {
		name        string
		reqBody     balanceWithdrawRequest
		contentType string
		mockData    mockData
		authUserID  int
		wantCode    int
	}{
		{
			name: "success",
			reqBody: balanceWithdrawRequest{
				OrderNum: orderNumByLuhn,
				Sum:      1,
			},
			mockData: mockData{
				err: nil,
			},
			authUserID: 1,
			wantCode:   http.StatusOK,
		},
		{
			name: "error not enough minerals",
			reqBody: balanceWithdrawRequest{
				OrderNum: orderNumByLuhn,
				Sum:      1,
			},
			mockData: mockData{
				err: model.ErrInsufficientPoints,
			},
			authUserID: 1,
			wantCode:   http.StatusPaymentRequired,
		},
		{
			name: "error empty order number",
			reqBody: balanceWithdrawRequest{
				OrderNum: "",
				Sum:      1,
			},
			mockData: mockData{
				err: model.ErrInsufficientPoints,
			},
			authUserID: 1,
			wantCode:   http.StatusBadRequest,
		},
		{
			name: "error invalid order number",
			reqBody: balanceWithdrawRequest{
				OrderNum: "123123",
				Sum:      1,
			},
			mockData: mockData{
				err: model.ErrInsufficientPoints,
			},
			authUserID: 1,
			wantCode:   http.StatusUnprocessableEntity,
		},
		{
			name:       "fail unauthenticated",
			mockData:   mockData{},
			authUserID: 0,
			wantCode:   http.StatusUnauthorized,
		},
		{
			name: "fail internal",
			reqBody: balanceWithdrawRequest{
				OrderNum: orderNumByLuhn,
				Sum:      1,
			},
			mockData: mockData{
				err: errors.New("db error"),
			},
			authUserID: 1,
			wantCode:   http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockBalanceService(ctrl)
			m.EXPECT().
				WithdrawPoints(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).
				Return(tc.mockData.err).
				AnyTimes()

			handler := NewBalanceWithdrawHandler(m)
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

			if tc.contentType == "" {
				tc.contentType = "application/json"
			}

			b, _ := json.Marshal(tc.reqBody)
			req, _ := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				"/",
				strings.NewReader(string(b)),
			)

			req.Header.Set("Content-Type", tc.contentType)

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
		})
	}
}
