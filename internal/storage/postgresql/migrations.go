package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

func runMigrations(ctx context.Context, conn *pgxpool.Pool) error {
	poolConn, err := conn.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("error creating pool connection: %w", err)
	}
	defer poolConn.Release()

	m, err := migrate.NewMigrator(ctx, poolConn.Conn(), "metrics_migrations")
	if err != nil {
		return fmt.Errorf("error migrations init: %w", err)
	}

	m.Migrations = []*migrate.Migration{
		{
			Sequence: 1,
			Name:     "init",
			UpSQL: `
			CREATE TABLE IF NOT EXISTS users (
				id SERIAL PRIMARY KEY,
				login VARCHAR(255) UNIQUE NOT NULL,
				password_hash VARCHAR(255) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			CREATE TABLE IF NOT EXISTS orders (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL REFERENCES users(id),
				number VARCHAR(255) UNIQUE NOT NULL,
				status VARCHAR(20) NOT NULL DEFAULT 'NEW',
				accrual BIGINT NOT NULL DEFAULT 0, -- stored in kopeks
				uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				last_polled_at TIMESTAMP WITH TIME ZONE
			);

			CREATE TABLE IF NOT EXISTS withdrawals (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL REFERENCES users(id),
				order_number VARCHAR(255) UNIQUE NOT NULL,
				sum BIGINT NOT NULL, -- stored in kopeks
				processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			CREATE INDEX IF NOT EXISTS idx_orders_user_id_status
			ON orders (user_id, status);

			CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id
			ON withdrawals (user_id);

			CREATE INDEX IF NOT EXISTS idx_orders_status_last_polled
			ON orders (status, last_polled_at);
			`,
			DownSQL: `
			DROP INDEX IF EXISTS idx_orders_user_id_status;
			DROP INDEX IF EXISTS idx_withdrawals_user_id;
			DROP INDEX IF EXISTS idx_orders_status_last_polled;

			DROP TABLE IF EXISTS users;
			DROP TABLE IF EXISTS orders;
			DROP TABLE IF EXISTS withdrawals;
			`,
		},
	}

	if err := m.Migrate(ctx); err != nil {
		return fmt.Errorf("error applying migrations: %w", err)
	}

	return nil
}
