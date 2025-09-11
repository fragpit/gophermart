package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragpit/gophermart/internal/model"
	"github.com/fragpit/gophermart/internal/utils/retry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ model.BalanceRepository = (*BalanceRepo)(nil)

type BalanceRepo struct {
	baseRepo
}

func (r *BalanceRepo) GetUserBalance(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	q := `
		SELECT (
			COALESCE((
				SELECT SUM(o.accrual) FROM orders o
				WHERE o.user_id = $1 AND o.status = 'PROCESSED'
			), 0)
			-
			COALESCE((
				SELECT SUM(w.sum) FROM withdrawals w
				WHERE w.user_id = $1
			), 0)
		)
		::bigint AS balance_kopeks
	`
	row := r.db.QueryRow(ctx, q, userID)

	var balance model.Kopek
	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

func (r *BalanceRepo) GetWithdrawalsSum(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	q := `
		SELECT COALESCE(SUM(sum), 0)::bigint as total_withdrawn_kopeks
		FROM withdrawals
		WHERE user_id = $1;
	`

	row := r.db.QueryRow(ctx, q, userID)

	var withdrawals model.Kopek
	if err := row.Scan(&withdrawals); err != nil {
		return 0, fmt.Errorf("failed to get withdrawals: %w", err)
	}

	return withdrawals, nil
}

func (r *BalanceRepo) WithdrawPoints(
	ctx context.Context,
	userID int,
	orderNum string,
	sum model.Kopek,
) error {
	txRetrier := retry.New(func(err error) bool {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "40001", "40P01":
				return true
			}
		}
		return false
	})

	op := func(ctx context.Context) error {
		tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: pgx.Serializable,
		})
		if err != nil {
			return fmt.Errorf("failed to start tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		q := `
			WITH bal AS (
			SELECT (
				COALESCE((
					SELECT SUM(o.accrual) FROM orders o
					WHERE o.user_id = $1 AND o.status = 'PROCESSED'
				), 0)
				-
				COALESCE((
					SELECT SUM(w.sum) FROM withdrawals w
					WHERE w.user_id = $1
				), 0)
			)
			::bigint AS balance
			),
			ins AS (
			INSERT INTO withdrawals (user_id, order_number, sum)
			SELECT
				$1,
				$2,
				$3::bigint
			FROM bal
			WHERE bal.balance >= $3::bigint
			RETURNING id
			)
			SELECT EXISTS(SELECT 1 FROM ins) AS ok;
		`

		var ok bool
		if err := tx.QueryRow(
			ctx,
			q,
			userID,
			orderNum,
			sum,
		).Scan(&ok); err != nil {
			return fmt.Errorf("withdraw exec: %w", err)
		}

		if !ok {
			return model.ErrInsufficientPoints
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit tx: %w", err)
		}
		return nil
	}

	if err := txRetrier.Do(ctx, op); err != nil {
		return fmt.Errorf("failed to retry: %w", err)
	}
	return nil
}
