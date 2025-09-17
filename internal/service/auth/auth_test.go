package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/fragpit/gophermart/internal/model"
	mocks "github.com/fragpit/gophermart/internal/service/auth/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestAuthService_Register(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type args struct {
		login    string
		password string
	}
	tests := []struct {
		name      string
		args      args
		prepare   func(*mocks.MockUsersRepository, context.Context, args)
		wantErr   error
		wantToken bool
	}{
		{
			name: "user already exists",
			args: args{"user", "pass"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).Return(&model.User{ID: 1}, nil)
			},
			wantErr:   model.ErrUserExists,
			wantToken: false,
		},
		{
			name: "invalid password policy",
			args: args{"user", "1"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).
					Return(nil, errors.New("not found"))
			},
			wantErr:   model.ErrPasswordPolicyViolated,
			wantToken: false,
		},
		{
			name: "create user and token",
			args: args{"user", "valid_password"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).
					Return(nil, errors.New("not found"))
				r.EXPECT().Create(ctx, gomock.Any()).
					Return(&model.User{ID: 42, Login: a.login}, nil)
			},
			wantErr:   nil,
			wantToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUsersRepository(ctrl)
			ctx := context.Background()
			svc := NewAuthService(repo, "secret", time.Minute)

			tt.prepare(repo, ctx, tt.args)

			token, err := svc.Register(ctx, tt.args.login, tt.args.password)

			assert.ErrorIs(t, err, tt.wantErr)

			if tt.wantToken {
				assert.NotEmpty(t, token)
			}
			if !tt.wantToken {
				assert.Empty(t, token)
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	type args struct {
		login    string
		password string
	}
	hashed, _ := HashPassword("pass")

	tests := []struct {
		name      string
		args      args
		prepare   func(*mocks.MockUsersRepository, context.Context, args)
		wantErr   error
		wantToken bool
	}{
		{
			name: "user not found",
			args: args{"user", "pass"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).
					Return(nil, errors.New("not found"))
			},
			wantErr:   model.ErrUserNotFound,
			wantToken: false,
		},
		{
			name: "invalid password",
			args: args{"user", "invalid_pass"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).
					Return(&model.User{ID: 10, Login: a.login, PasswordHash: hashed}, nil)
			},
			wantErr:   model.ErrInvalidCredentials,
			wantToken: false,
		},
		{
			name: "success",
			args: args{"user", "pass"},
			prepare: func(r *mocks.MockUsersRepository, ctx context.Context, a args) {
				r.EXPECT().GetByLogin(ctx, a.login).
					Return(&model.User{ID: 7, Login: a.login, PasswordHash: hashed}, nil)
			},
			wantErr:   nil,
			wantToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUsersRepository(ctrl)
			ctx := context.Background()
			svc := NewAuthService(repo, "secret", time.Minute)

			tt.prepare(repo, ctx, tt.args)

			token, err := svc.Login(ctx, tt.args.login, tt.args.password)

			assert.ErrorIs(t, err, tt.wantErr)

			if tt.wantToken {
				assert.NotEmpty(t, token)
			}
			if !tt.wantToken {
				assert.Empty(t, token)
			}
		})
	}
}
