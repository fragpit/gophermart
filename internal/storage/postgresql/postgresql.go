package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragpit/gophermart/internal/healthcheck"
	"github.com/fragpit/gophermart/internal/model"
	"github.com/fragpit/gophermart/internal/utils/retry"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	retrier := retry.New(isRetryable)

	return &Storage{
		DB:      db,
		retrier: retrier,
	}, nil
}

var _ healthcheck.HealthRepository = (*Storage)(nil)

func (s *Storage) Ping(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("database connection not initialized")
	}

	op := func(ctx context.Context) error {
		return s.DB.Ping(ctx)
	}

	if err := s.retrier.Do(ctx, op); err != nil {
		return err
	}

	return nil
}

var _ model.UserRepository = (*Storage)(nil)

func (s *Storage) Create(
	ctx context.Context,
	user *model.User,
) (*model.User, error) {
	q := `
	INSERT INTO users (login, password_hash)
	VALUES (@login, @password_hash)
	RETURNING id;
	`

	args := pgx.NamedArgs{
		"login":         user.Login,
		"password_hash": user.PasswordHash,
	}

	var id int32
	row := s.DB.QueryRow(ctx, q, args)
	if err := row.Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	user.ID = int(id)

	return user, nil
}

func (s *Storage) GetByLogin(
	ctx context.Context,
	login string,
) (*model.User, error) {
	q := `SELECT id, login, password_hash FROM users WHERE login = $1`

	var (
		userID    int
		userLogin string
		userPHash string
	)
	row := s.DB.QueryRow(ctx, q, login)
	if err := row.Scan(&userID, &userLogin, &userPHash); err != nil {
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	u := &model.User{
		ID:           userID,
		Login:        userLogin,
		PasswordHash: userPHash,
	}

	return u, nil
}
