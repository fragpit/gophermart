package model

import (
	"context"
	"time"
)

type WithdrawalsRepository interface {
	GetWithdrawalsByUserID(ctx context.Context, userID int) ([]Withdrawal, error)
}

type Withdrawal struct {
	ID          int
	UserID      int
	OrderNum    string
	Sum         Kopek
	ProcessedAt time.Time
}

func ValidateSum(sum Kopek) bool {
	return sum > 0
}
