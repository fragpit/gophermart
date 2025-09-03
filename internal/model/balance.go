package model

import (
	"context"
	"errors"
)

var (
	ErrInsufficientPoints = errors.New("insufficient points")
)

type BalanceRepository interface {
	GetTotalPoints(ctx context.Context, userID int) (int, error)
	GetWithdrawals(ctx context.Context, userID int) (int, error)
	WithdrawPoints(
		ctx context.Context,
		userID int,
		orderNum string,
		sum int,
	) error
}
