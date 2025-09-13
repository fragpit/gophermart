package model

import (
	"context"
	"errors"
)

var (
	ErrInsufficientPoints = errors.New("insufficient points")
)

type BalanceRepository interface {
	GetUserBalance(ctx context.Context, userID int) (Kopek, error)
	GetWithdrawalsSum(ctx context.Context, userID int) (Kopek, error)
	WithdrawPoints(
		ctx context.Context,
		userID int,
		orderNum string,
		sum Kopek,
	) error
}
