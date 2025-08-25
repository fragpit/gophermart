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
					accrual DECIMAL(10,2),
					uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			CREATE TABLE IF NOT EXISTS withdrawals (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id),
					order_number VARCHAR(255) NOT NULL,
					sum DECIMAL(10,2) NOT NULL,
					processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			`,
			DownSQL: `
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
