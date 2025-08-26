package auth

import (
	"context"

	"github.com/fragpit/gophermart/internal/api/handlers"
	"github.com/fragpit/gophermart/internal/model"
)

var _ handlers.AuthService = (*AuthService)(nil)

type AuthService struct {
	repo model.UserRepository
}

func NewAuthService(repo model.UserRepository) *AuthService {
	return &AuthService{
		repo: repo,
	}
}

func (a *AuthService) Register(
	ctx context.Context,
	login, password string,
) (string, error) {
	return "1111", nil
}

func (a *AuthService) Login(
	ctx context.Context,
	login, password string,
) error {
	return nil
}
