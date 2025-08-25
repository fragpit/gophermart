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
					id TEXT PRIMARY KEY,
					login TEXT NOT NULL,
					password_hash TEXT NOT NULL
			);

			CREATE TABLE IF NOT EXISTS orders (
					id TEXT PRIMARY KEY,
					user_id TEXT NOT NULL,
					number INT NOT NULL,
					status TEXT NOT NULL
			);

			CREATE TABLE IF NOT EXISTS withdrawals (
					id TEXT PRIMARY KEY,
					user_id TEXT NOT NULL,
					order_number INT NOT NULL,
					sum F
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
