package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragpit/gophermart/internal/utils/retry"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// var _ repository.Repository = (*Storage)(nil)

type Storage struct {
	DB      *pgxpool.Pool
	retrier *retry.Retrier
}

func NewStorage(ctx context.Context, dbDSN string) (*Storage, error) {
	db, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		return nil, fmt.Errorf("error creating pgxpool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping error: %w", err)
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

	retrier := retry.New(isRetryable)

	return &Storage{
		DB:      db,
		retrier: retrier,
	}, nil
}
