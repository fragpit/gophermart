package healthcheck

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	mocks "github.com/fragpit/gophermart/internal/service/healthcheck/mocks"
	"go.uber.org/mock/gomock"
)

func TestHealthService_Check(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	tests := []struct {
		name    string
		pingErr error
	}{
		{name: "success", pingErr: nil},
		{name: "fail", pingErr: errors.New("ping error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockHealthRepository(ctrl)
			svc := NewHealthcheckService(repo)
			ctx := context.Background()

			repo.EXPECT().Ping(ctx).Return(tt.pingErr)

			err := svc.Check(ctx)
			if !errors.Is(err, tt.pingErr) {
				t.Fatalf(
					"unexpected error: got %v, want %v",
					err,
					tt.pingErr,
				)
			}
		})
	}
}
