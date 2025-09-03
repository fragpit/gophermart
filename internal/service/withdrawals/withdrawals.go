package withdrawals

import (
	"context"

	"github.com/fragpit/gophermart/internal/model"
)

// var _ handlers.WithdrawalsService = (*WithdrawalsService)(nil)

type WithdrawalsService struct {
	repo model.WithdrawalsRepository
}

func NewWithdrawalsService(
	repo model.WithdrawalsRepository,
) *WithdrawalsService {
	return &WithdrawalsService{
		repo: repo,
	}
}

func (o *WithdrawalsService) GetWithdrawalsByUser(
	ctx context.Context,
	userID int,
) ([]model.Withdrawal, error) {
	return o.repo.GetWithdrawalsByUserID(ctx, userID)
}
