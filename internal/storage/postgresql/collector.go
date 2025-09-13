package postgresql

import (
	"context"
	"fmt"

	"github.com/fragpit/gophermart/internal/model"
	collector "github.com/fragpit/gophermart/internal/service/accrual-collector"
)

var _ collector.CollectorRepository = (*CollectorRepo)(nil)

type CollectorRepo struct {
	baseRepo
}

func (r *CollectorRepo) SetAccrual(
	ctx context.Context,
	id int,
	sum model.Kopek,
) error {
	q := `
		UPDATE orders
		SET accrual = $1,
			status = $2
		WHERE id = $3
	`

	if _, err := r.db.Exec(ctx, q, sum, model.StatusProcessed, id); err != nil {
		return fmt.Errorf("failed to update accrual: %w", err)
	}

	return nil
}

func (r *CollectorRepo) SetStatus(
	ctx context.Context,
	id int,
	status string,
) error {
	q := `
		UPDATE orders
		SET status = $1
		WHERE id = $2
	`

	if _, err := r.db.Exec(ctx, q, status, id); err != nil {
		return fmt.Errorf("failed to set order status: %w", err)
	}

	return nil
}

func (r *CollectorRepo) GetOrdersBatch(
	ctx context.Context,
	batchSize int,
) ([]model.Order, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qSelect := `
		SELECT id FROM orders
		WHERE status IN ('NEW', 'PROCESSING')
		ORDER BY last_polled_at NULLS FIRST, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, qSelect, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query tx: %w", err)
	}

	var ids []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	if len(ids) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("failed to commit tx: %w", err)
		}
		return nil, nil
	}

	qUpdate := `
		UPDATE orders AS o
		SET last_polled_at = NOW()
		WHERE o.id = ANY($1)
		RETURNING
			o.id,
			o.user_id,
			o.number,
			o.status,
			o.accrual,
			o.uploaded_at
	`

	rows2, err := tx.Query(ctx, qUpdate, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query tx: %w", err)
	}

	var orders []model.Order
	for rows2.Next() {
		var o model.Order
		if err := rows2.Scan(
			&o.ID,
			&o.UserID,
			&o.Number,
			&o.Status,
			&o.Accrual,
			&o.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		orders = append(orders, o)
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	return orders, nil
}
