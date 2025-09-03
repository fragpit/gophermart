package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragpit/gophermart/internal/model"
	"github.com/fragpit/gophermart/internal/service/healthcheck"
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

var _ model.OrdersRepository = (*Storage)(nil)

func (s *Storage) GetOrdersByUserID(
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
		accrual     int
		uploadedAt  time.Time
	)
	rows, err := s.DB.Query(ctx, q, userID)
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

func (s *Storage) AddOrder(
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
	if _, err := s.DB.Exec(ctx, q, args); err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

var _ model.BalanceRepository = (*Storage)(nil)

func (s *Storage) GetTotalPoints(ctx context.Context, userID int) (int, error) {
	q := `
	SELECT COALESCE(SUM(accrual), 0) AS total_accrual
	FROM orders
	WHERE user_id = $1 AND status = 'PROCESSED';
	`

	row := s.DB.QueryRow(ctx, q, userID)

	var balance int
	if err := row.Scan(&balance); err != nil {
		return 0, err
	}

	return balance, nil
}
func (s *Storage) GetWithdrawals(ctx context.Context, userID int) (int, error) {
	q := `
	SELECT COALESCE(SUM(sum), 0) as total_withdrawn
	FROM withdrawals
	WHERE user_id = $1;
	`

	row := s.DB.QueryRow(ctx, q, userID)

	var withdrawn int
	if err := row.Scan(&withdrawn); err != nil {
		return 0, err
	}

	return withdrawn, nil
}

func (s *Storage) WithdrawPoints(
	ctx context.Context,
	userID int,
	orderNum string,
	sum int,
) error {
	q := `
	INSERT INTO withdrawals (user_id, order_number, sum)
	VALUES (@userID, @orderNum, @sum)
	`

	args := pgx.NamedArgs{
		"userID":   userID,
		"orderNum": orderNum,
		"sum":      sum,
	}
	if _, err := s.DB.Exec(ctx, q, args); err != nil {
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return nil
}

var _ model.WithdrawalsRepository = (*Storage)(nil)

func (s *Storage) GetWithdrawalsByUserID(
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
		sum         int
		processedAt time.Time
	)
	rows, err := s.DB.Query(ctx, q, userID)
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
