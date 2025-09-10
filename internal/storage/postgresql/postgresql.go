package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragpit/gophermart/internal/model"
	collector "github.com/fragpit/gophermart/internal/service/accrual-collector"
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
	q := `
		SELECT id, login, password_hash
		FROM users
		WHERE login = $1
	`

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
		SELECT id, number, status, (accrual * 100)::bigint
			AS accrual_kopeks, uploaded_at
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			var existingUserID int
			row := s.DB.QueryRow(
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

var _ model.BalanceRepository = (*Storage)(nil)

func (s *Storage) GetUserBalance(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	q := `
		SELECT
		COALESCE((
			SELECT SUM((o.accrual * 100)::bigint)
			FROM orders o
			WHERE o.user_id = $1 AND o.status = 'PROCESSED'
		), 0)
		-
		COALESCE((
			SELECT SUM((w.sum * 100)::bigint)
			FROM withdrawals w
			WHERE w.user_id = $1
		), 0) AS balance_kopeks
	`
	row := s.DB.QueryRow(ctx, q, userID)

	var balance model.Kopek
	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

func (s *Storage) GetWithdrawalsSum(
	ctx context.Context,
	userID int,
) (model.Kopek, error) {
	q := `
		SELECT COALESCE(SUM((sum * 100)::bigint), 0) as total_withdrawn_kopeks
		FROM withdrawals
		WHERE user_id = $1;
	`

	row := s.DB.QueryRow(ctx, q, userID)

	var withdrawals model.Kopek
	if err := row.Scan(&withdrawals); err != nil {
		return 0, fmt.Errorf("failed to get withdrawals: %w", err)
	}

	return withdrawals, nil
}

func (s *Storage) WithdrawPoints(
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
		tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: pgx.Serializable,
		})
		if err != nil {
			return fmt.Errorf("failed to start tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		q := `
			WITH bal AS (
			SELECT
				COALESCE((
				SELECT SUM((o.accrual * 100)::bigint)
				FROM orders o
				WHERE o.user_id = $1 AND o.status = 'PROCESSED'
				), 0)
				-
				COALESCE((
				SELECT SUM((w.sum * 100)::bigint)
				FROM withdrawals w
				WHERE w.user_id = $1
				), 0) AS balance
			),
			ins AS (
			INSERT INTO withdrawals (user_id, order_number, sum)
			SELECT
				$1,
				$2,
				($3::numeric / 100.0)
			FROM bal
			WHERE bal.balance >= $3
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

var _ model.WithdrawalsRepository = (*Storage)(nil)

func (s *Storage) GetWithdrawalsByUserID(
	ctx context.Context,
	userID int,
) ([]model.Withdrawal, error) {
	q := `
		SELECT id, order_number, (sum * 100)::bigint AS sum_kopeks, processed_at
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

var _ collector.CollectorRepository = (*Storage)(nil)

func (s *Storage) SetAccrual(
	ctx context.Context,
	id int,
	sum model.Kopek,
) error {
	q := `
		UPDATE orders
		SET accrual = ($1::numeric / 100.0),
			status = $2
		WHERE id = $3
	`

	if _, err := s.DB.Exec(ctx, q, sum, model.StatusProcessed, id); err != nil {
		return fmt.Errorf("failed to update accrual: %w", err)
	}

	return nil
}

func (s *Storage) SetStatus(
	ctx context.Context,
	id int,
	status string,
) error {
	q := `
		UPDATE orders
		SET status = $1
		WHERE id = $2
	`

	if _, err := s.DB.Exec(ctx, q, status, id); err != nil {
		return fmt.Errorf("failed to set order status: %w", err)
	}

	return nil
}

func (s *Storage) GetOrdersBatch(
	ctx context.Context,
	batchSize int,
) ([]model.Order, error) {
	tx, err := s.DB.Begin(ctx)
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
			(o.accrual * 100)::bigint AS accrual_kopeks,
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
