package balance

import (
	"context"

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

func (b *BalanceService) GetUserBalance(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	return b.repo.GetUserBalance(ctx, userID)
}

func (b *BalanceService) GetWithdrawalsSum(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	return b.repo.GetWithdrawalsSum(ctx, userID)
}

func (b *BalanceService) WithdrawPoints(
	ctx context.Context,
	userID int,
	orderNum string,
	sum model.Kopek,
) error {
	return b.repo.WithdrawPoints(ctx, userID, orderNum, sum)
}
