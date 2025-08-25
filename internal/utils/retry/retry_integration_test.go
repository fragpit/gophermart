//go:build integration

package retry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dsnFormat = "postgresql://test:test@localhost:%s/test?sslmode=disable"

func createPostgreSQLIsRetryable() IsRetryableFunc {
	return func(err error) bool {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgerrcode.IsConnectionException(pgErr.Code) ||
				pgerrcode.IsOperatorIntervention(pgErr.Code)
		}

		var connErr *pgconn.ConnectError
		return errors.As(err, &connErr)
	}
}

func TestRetrier_PostgreSQL_Integration_PingSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	pgContainer, err := runPostgresContainer()
	require.NoError(t, err)
	defer pgContainer.Close()

	dsn := fmt.Sprintf(dsnFormat, pgContainer.GetPort("5432/tcp"))

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Cannot connect to PostgreSQL database: %v", err)
	}
	defer db.Close()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable)

	op := func(ctx context.Context) error {
		return db.Ping(ctx)
	}

	err = retrier.Do(ctx, op)
	assert.NoError(t, err)
}

func TestRetrier_PostgreSQL_Integration_PingWithInvalidDSN(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	invalidDSN := "postgresql://invalid:invalid@localhost:9999/invalid?sslmode=disable"

	db, err := pgxpool.New(ctx, invalidDSN)
	if err != nil {
		isRetryable := createPostgreSQLIsRetryable()

		shouldRetry := isRetryable(err)

		t.Logf("Connection creation error: %v, retryable: %v", err, shouldRetry)
		return
	}
	defer db.Close()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable, WithBackoff([]time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
	}))

	op := func(ctx context.Context) error {
		return db.Ping(ctx)
	}

	start := time.Now()
	err = retrier.Do(ctx, op)
	duration := time.Since(start)

	assert.Error(t, err)
	if isRetryable(err) {
		assert.Contains(t, err.Error(), "operation failed after retries")
		assert.GreaterOrEqual(t, duration, 3*time.Millisecond)
	}
}

func TestRetrier_PostgreSQL_Integration_PingContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	pgContainer, err := runPostgresContainer()
	require.NoError(t, err)
	defer pgContainer.Close()

	dsn := fmt.Sprintf(dsnFormat, pgContainer.GetPort("5432/tcp"))

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Cannot create database connection for timeout test: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable)

	op := func(ctx context.Context) error {
		return db.Ping(ctx)
	}

	err = retrier.Do(ctx, op)

	if err != nil {
		t.Logf("Ping failed as expected due to timeout: %v", err)
	} else {
		t.Log("Ping succeeded despite very short timeout")
	}
}

func TestRetrier_PostgreSQL_Integration_MultipleConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable)

	pgContainer, err := runPostgresContainer()
	require.NoError(t, err)
	defer pgContainer.Close()

	dsn := fmt.Sprintf(dsnFormat, pgContainer.GetPort("5432/tcp"))

	const numConnections = 5
	errors := make(chan error, numConnections)

	for i := 0; i < numConnections; i++ {
		go func() {
			db, err := pgxpool.New(ctx, dsn)
			if err != nil {
				errors <- err
				return
			}
			defer db.Close()

			op := func(ctx context.Context) error {
				return db.Ping(ctx)
			}

			errors <- retrier.Do(ctx, op)
		}()
	}

	var successCount int
	for i := 0; i < numConnections; i++ {
		err := <-errors
		if err == nil {
			successCount++
		} else {
			t.Logf("Connection %d failed: %v", i, err)
		}
	}

	if successCount == 0 {
		t.Skip("All connections failed - PostgreSQL may not be available")
	}

	t.Logf(
		"Successfully completed %d out of %d connections",
		successCount,
		numConnections,
	)
}

func TestRetrier_PostgreSQL_Integration_DatabaseReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	pgContainer, err := runPostgresContainer()
	require.NoError(t, err)
	defer pgContainer.Close()

	dsn := fmt.Sprintf(dsnFormat, pgContainer.GetPort("5432/tcp"))

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Cannot create database connection: %v", err)
	}
	defer db.Close()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable, WithBackoff([]time.Duration{
		5 * time.Millisecond,
		10 * time.Millisecond,
	}))

	op := func(ctx context.Context) error {
		return db.Ping(ctx)
	}

	err = retrier.Do(ctx, op)
	if err != nil {
		t.Skipf("Initial ping failed: %v", err)
	}

	db.Close()

	err = retrier.Do(ctx, op)
	assert.Error(t, err, "Ping should fail on closed connection")
}

func BenchmarkRetrier_PostgreSQL_Integration_Ping(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping integration benchmark in short mode")
	}

	ctx := context.Background()

	pgContainer, err := runPostgresContainer()
	require.NoError(b, err)
	defer pgContainer.Close()

	dsn := fmt.Sprintf(dsnFormat, pgContainer.GetPort("5432/tcp"))

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		b.Skipf("Cannot connect to PostgreSQL database: %v", err)
	}
	defer db.Close()

	isRetryable := createPostgreSQLIsRetryable()
	retrier := New(isRetryable)

	op := func(ctx context.Context) error {
		return db.Ping(ctx)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retrier.Do(ctx, op)
	}
}

func runPostgresContainer() (*dockertest.Resource, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("error constructing pool: %w", err)
	}

	postgres, err := pool.Run("postgres", "17", []string{
		"POSTGRES_DB=test",
		"POSTGRES_USER=test",
		"POSTGRES_PASSWORD=test",
		// "listen_addresses = '*'",
	})
	if err != nil {
		return nil, fmt.Errorf("error running container: %w", err)
	}

	time.Sleep(2 * time.Second)

	info, err := pool.Client.InspectContainer(postgres.Container.ID)
	if err != nil {
		logs := getContainerLogs(pool, postgres.Container.ID)
		return nil, fmt.Errorf(
			"container inspect failed: %w\nlogs:\n%s",
			err,
			logs,
		)
	}

	if info.State.ExitCode != 0 {
		logs := getContainerLogs(pool, postgres.Container.ID)
		return nil, fmt.Errorf(
			"container exited with status: %d\nlogs:\n%s",
			info.State.ExitCode,
			logs,
		)
	}

	return postgres, nil
}

// getContainerLogs returns last lines of the container logs to aid debugging.
func getContainerLogs(pool *dockertest.Pool, id string) string {
	var buf bytes.Buffer
	// Tail some lines to keep error output concise.
	opts := docker.LogsOptions{
		Container:    id,
		Stdout:       true,
		Stderr:       true,
		Follow:       false,
		Timestamps:   false,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Tail:         "200",
	}
	if err := pool.Client.Logs(opts); err != nil {
		return fmt.Sprintf("failed to get logs: %v", err)
	}
	return buf.String()
}
