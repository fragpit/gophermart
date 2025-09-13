package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragpit/gophermart/internal/model"
	collector "github.com/fragpit/gophermart/internal/service/accrual-collector"
	"github.com/fragpit/gophermart/internal/service/healthcheck"
	"github.com/fragpit/gophermart/internal/utils/retry"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type baseRepo struct {
	db      *pgxpool.Pool
	retrier *retry.Retrier
}

type Repositories struct {
	Health      healthcheck.HealthRepository
	Users       model.UsersRepository
	Orders      model.OrdersRepository
	Balance     model.BalanceRepository
	Withdrawals model.WithdrawalsRepository
	Collector   collector.CollectorRepository
}

func NewStorage(ctx context.Context, dbDSN string) (*Repositories, error) {
	db, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		return nil, fmt.Errorf("error creating pgxpool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping error: %w", err)
	}

	_, err = db.Exec(ctx, "SET timezone = 'UTC'")
	if err != nil {
		return nil, fmt.Errorf("error setting timezone to UTC: %w", err)
	}

	if err := runMigrations(ctx, db); err != nil {
		return nil, fmt.Errorf("error running migrations: %w", err)
	}

	isRetryable := func(err error) bool {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgerrcode.IsConnectionException(pgErr.Code) ||
				pgerrcode.IsOperatorIntervention(pgErr.Code)
		}

		var connErr *pgconn.ConnectError
		return errors.As(err, &connErr)
	}

	b := baseRepo{
		db:      db,
		retrier: retry.New(isRetryable),
	}

	repos := &Repositories{
		Health:      &HealthRepo{baseRepo: b},
		Users:       &UsersRepo{baseRepo: b},
		Orders:      &OrdersRepo{baseRepo: b},
		Balance:     &BalanceRepo{baseRepo: b},
		Withdrawals: &WithdrawalsRepo{baseRepo: b},
		Collector:   &CollectorRepo{baseRepo: b},
	}
	return repos, nil
}
