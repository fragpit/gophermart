package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_handlers "github.com/fragpit/gophermart/internal/api/handlers/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestHealthHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock_handlers.NewMockHealthService(ctrl)

	tests := []struct {
		name        string
		returnError error
		wantCode    int
	}{
		{
			name:        "success",
			returnError: nil,
			wantCode:    http.StatusOK,
		},
		{
			name:        "fail",
			returnError: errors.New("test error"),
			wantCode:    http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m.EXPECT().Check(context.Background()).Return(tc.returnError)

			handler := NewHealthHandler(m)
			rec := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
		})
	}
}
