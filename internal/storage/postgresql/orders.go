package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragpit/gophermart/internal/model"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ model.OrdersRepository = (*OrdersRepo)(nil)

type OrdersRepo struct {
	baseRepo
}

func (r *OrdersRepo) GetOrdersByUserID(
	ctx context.Context,
	userID int,
) ([]model.Order, error) {
	q := `
		SELECT id, number, status, accrual, uploaded_at
		FROM orders
		WHERE user_id = $1
		ORDER BY id DESC
	`

	var (
		orderID     int
		orderNumber string
		status      model.OrderStatus
		accrual     model.Kopek
		uploadedAt  time.Time
	)
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("orders query error: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		if err := rows.Scan(
			&orderID,
			&orderNumber,
			&status,
			&accrual,
			&uploadedAt,
		); err != nil {
			return nil, fmt.Errorf("error reading values: %w", err)
		}
		order := model.Order{
			ID:         orderID,
			UserID:     userID,
			Number:     orderNumber,
			Status:     status,
			Accrual:    accrual,
			UploadedAt: uploadedAt,
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func (r *OrdersRepo) AddOrder(
	ctx context.Context,
	order *model.Order,
) error {
	q := `
		INSERT INTO orders (user_id, number, status, accrual)
		VALUES (@userID, @orderNumber, @orderStatus, @accrual)
	`

	args := pgx.NamedArgs{
		"userID":      order.UserID,
		"orderNumber": order.Number,
		"orderStatus": order.Status,
		"accrual":     order.Accrual,
	}
	if _, err := r.db.Exec(ctx, q, args); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			var existingUserID int
			row := r.db.QueryRow(
				ctx,
				`SELECT user_id FROM orders WHERE number = $1`,
				order.Number,
			)
			if scanErr := row.Scan(&existingUserID); scanErr != nil {
				return fmt.Errorf(
					"%w: order exists; failed to get owner: %v",
					model.ErrOrderAlreadyExist,
					scanErr,
				)
			}
			if existingUserID != order.UserID {
				return model.ErrOrderAlreadyAddedByOtherUser
			}
			return model.ErrOrderAlreadyExist
		}

		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}
