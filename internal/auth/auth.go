package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fragpit/gophermart/internal/api/handlers"
	"github.com/fragpit/gophermart/internal/model"
)

var _ handlers.AuthService = (*AuthService)(nil)

type AuthService struct {
	repo model.UserRepository

	jwtSecret string
	jwtTTL    time.Duration
}

func NewAuthService(
	repo model.UserRepository,
	jwtSecret string,
	jwtTTL time.Duration,
) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}
}

func (a *AuthService) Register(
	ctx context.Context,
	login, password string,
) (string, error) {
	if _, err := a.repo.GetByLogin(ctx, login); err == nil {
		return "", model.ErrUserExists
	}

	if err := model.ValidatePassword(password); err != nil {
		return "", model.ErrPasswordPolicyViolated
	}

	u := model.NewUser(login)
	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	u.PasswordHash = passwordHash

	u, err = a.repo.Create(ctx, u)
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	slog.Info("user created", slog.Int("user_id", u.ID))

	token, err := CreateJWTToken(a.jwtSecret, a.jwtTTL, u.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

func (a *AuthService) Login(
	ctx context.Context,
	login, password string,
) (string, error) {
	u, err := a.repo.GetByLogin(ctx, login)
	if err != nil {
		return "", model.ErrUserNotFound
	}

	if ok := ComparePasswordHash(password, u.PasswordHash); !ok {
		return "", model.ErrInvalidCredentials
	}

	token, err := CreateJWTToken(a.jwtSecret, a.jwtTTL, u.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return token, nil
}
