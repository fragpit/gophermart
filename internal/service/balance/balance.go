package balance

import (
	"context"
	"fmt"

	"github.com/fragpit/gophermart/internal/api/handlers"
	"github.com/fragpit/gophermart/internal/model"
)

var _ handlers.BalanceService = (*BalanceService)(nil)

type BalanceService struct {
	repo model.BalanceRepository
}

func NewBalanceService(repo model.BalanceRepository) *BalanceService {
	return &BalanceService{
		repo: repo,
	}
}

func (b *BalanceService) GetTotalPoints(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	return b.repo.GetTotalPoints(ctx, userID)
}

func (b *BalanceService) GetWithdrawals(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	return b.repo.GetWithdrawals(ctx, userID)
}

func (o *BalanceService) WithdrawPoints(
	ctx context.Context,
	userID int,
	orderNum string,
	sum model.Kopek,
) error {
	totalWithdrawals, err := o.GetWithdrawals(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get withdrawals: %w", err)
	}

	totalPoints, err := o.GetTotalPoints(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get total points: %w", err)
	}

	balance := totalPoints - totalWithdrawals
	if balance < sum {
		return model.ErrInsufficientPoints
	}

	return o.repo.WithdrawPoints(ctx, userID, orderNum, sum)
}
