package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mock_handlers "github.com/fragpit/gophermart/internal/api/handlers/mocks"
	"github.com/fragpit/gophermart/internal/api/middleware"
	"github.com/fragpit/gophermart/internal/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestWithdrawalsHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type mockData struct {
		wd  []model.Withdrawal
		err error
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
				wd: []model.Withdrawal{
					{
						ID:          1,
						UserID:      1,
						OrderNum:    orderNumByLuhn,
						Sum:         1,
						ProcessedAt: time.Now(),
					},
				},
				err: nil,
			},
			authUserID: 1,
			wantCode:   http.StatusOK,
		},
		{
			name: "success empty",
			mockData: mockData{
				wd:  []model.Withdrawal{},
				err: nil,
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

			m := mock_handlers.NewMockWithdrawalsService(ctrl)
			m.EXPECT().
				GetWithdrawalsByUser(gomock.Any(), gomock.Any()).
				Return(tc.mockData.wd, tc.mockData.err).
				AnyTimes()

			handler := NewWithdrawalsHandler(m)
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
