package healthcheck

import (
	"context"
)

type HealthRepository interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	repo HealthRepository
}

func NewHealthcheckService(repo HealthRepository) *HealthService {
	return &HealthService{
		repo: repo,
	}
}

func (h *HealthService) Check(ctx context.Context) error {
	return h.repo.Ping(ctx)
}
