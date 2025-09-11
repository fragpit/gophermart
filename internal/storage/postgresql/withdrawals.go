package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/fragpit/gophermart/internal/model"
)

var _ model.WithdrawalsRepository = (*WithdrawalsRepo)(nil)

type WithdrawalsRepo struct {
	baseRepo
}

func (r *WithdrawalsRepo) GetWithdrawalsByUserID(
	ctx context.Context,
	userID int,
) ([]model.Withdrawal, error) {
	q := `
		SELECT id, order_number, sum, processed_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY id DESC
	`

	var (
		orderID     int
		orderNumber string
		sum         model.Kopek
		processedAt time.Time
	)
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("withdrawals query error: %w", err)
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		if err := rows.Scan(
			&orderID,
			&orderNumber,
			&sum,
			&processedAt,
		); err != nil {
			return nil, fmt.Errorf("error reading values: %w", err)
		}
		withdrawal := model.Withdrawal{
			ID:          orderID,
			UserID:      userID,
			OrderNum:    orderNumber,
			Sum:         sum,
			ProcessedAt: processedAt,
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	return withdrawals, nil
}
