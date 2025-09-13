package postgresql

import (
	"context"
	"fmt"

	"github.com/fragpit/gophermart/internal/service/healthcheck"
)

var _ healthcheck.HealthRepository = (*HealthRepo)(nil)

type HealthRepo struct {
	baseRepo
}

func (r *HealthRepo) Ping(ctx context.Context) error {
	if r.db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	op := func(ctx context.Context) error {
		return r.db.Ping(ctx)
	}

	if err := r.retrier.Do(ctx, op); err != nil {
		return err
	}

	return nil
}
