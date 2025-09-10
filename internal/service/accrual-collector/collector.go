package collector

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fragpit/gophermart/internal/model"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
)

const (
	clientTimeout           = 5 * time.Second
	getOrdersURL            = "/api/orders/"
	defaultRetryAfterPeriod = "60"
)

type CollectorRepository interface {
	SetAccrual(ctx context.Context, id int, sum model.Kopek) error
	SetStatus(ctx context.Context, id int, status string) error
	GetOrdersBatch(ctx context.Context, batchSize int) ([]model.Order, error)
}

type AccrualResponse struct {
	Number  string      `json:"order"`
	Status  string      `json:"status"`
	Accrual model.Kopek `json:"accrual,omitempty"`
}

type Collector struct {
	PollInterval time.Duration
	Client       *resty.Client

	repo        CollectorRepository
	nextAllowed atomic.Int64

	WorkersNum int
	BatchSize  int
}

func NewCollector(
	accrualAddress string,
	interval time.Duration,

	repo CollectorRepository,
) *Collector {
	client := resty.New()

	client.
		SetTimeout(clientTimeout).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		SetBaseURL(accrualAddress)

	c := &Collector{
		PollInterval: interval,
		Client:       client,
		repo:         repo,
		BatchSize:    10,
		WorkersNum:   3,
	}
	c.nextAllowed.Store(time.Now().UnixNano())

	return c
}

func (c *Collector) Run(ctx context.Context) error {
	tick := time.NewTicker(c.PollInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if ctx.Err() != nil {
				continue
			}

			if time.Now().UnixNano() < c.nextAllowed.Load() {
				continue
			}

			slog.Info("fetching accrual data")
			if err := c.processOrders(ctx); err != nil {
				return fmt.Errorf("collector error: %w", err)
			}
		}
	}
}

func (c *Collector) processOrders(ctx context.Context) error {
	orders, err := c.repo.GetOrdersBatch(ctx, c.BatchSize)
	if err != nil {
		return err
	}
	slog.Debug("fetched orders", slog.Int("count", len(orders)))
	if len(orders) == 0 {
		return nil
	}

	jobs := make(chan model.Order, c.BatchSize)
	worker := func() error {
		for j := range jobs {
			if ctx.Err() != nil {
				return nil
			}

			if time.Now().UnixNano() < c.nextAllowed.Load() {
				return nil
			}

			if err := c.handleOrder(ctx, &j); err != nil {
				return err
			}
		}

		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < c.WorkersNum; i++ {
		g.Go(func() error {
			return worker()
		})
	}

Loop:
	for _, o := range orders {
		if ctx.Err() != nil {
			break
		}

		if time.Now().UnixNano() < c.nextAllowed.Load() {
			break
		}
		select {
		case jobs <- o:
		case <-ctx.Done():
			break Loop
		}
	}

	close(jobs)
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (c *Collector) handleOrder(ctx context.Context, order *model.Order) error {
	if time.Now().UnixNano() < c.nextAllowed.Load() {
		return nil
	}

	slog.Info("processing order", slog.String("number", order.Number))

	var respBody AccrualResponse
	resp, err := c.Client.R().
		SetContext(ctx).
		SetResult(&respBody).
		Get(getOrdersURL + order.Number)
	if err != nil {
		slog.Error("failed to request accrual", slog.Any("error", err))
		return fmt.Errorf("failed to request accrual: %w", err)
	}

	sc := resp.StatusCode()
	if sc != http.StatusOK {
		switch sc {
		case http.StatusNoContent:
			slog.Info(
				"order is not registered in accrual",
				slog.String("number", order.Number),
			)
			return nil
		case http.StatusTooManyRequests:
			periodRaw := strings.TrimSpace(resp.Header().Get("Retry-After"))
			if periodRaw == "" {
				periodRaw = defaultRetryAfterPeriod
			}
			period, err := strconv.Atoi(periodRaw)
			if err != nil {
				slog.Error(
					"failed to get retry period from header, setting default",
					slog.String("header_value", periodRaw),
					slog.String("default_value", defaultRetryAfterPeriod),
				)
				period, _ = strconv.Atoi(defaultRetryAfterPeriod)
			}

			d := time.Duration(period) * time.Second
			c.setRetryAfter(d)
			slog.Info(
				"too many requests to accrual, setting retry-after",
				slog.Duration("period", d),
			)

			return nil
		default:
			slog.Error("failed to request accrual", slog.Int("http_code", sc))
			return fmt.Errorf("failed to request accrual, http_code=%d", sc)
		}
	}

	fmt.Println(respBody)

	switch respBody.Status {
	case model.StatusProcessed.String():
		if err := c.repo.SetAccrual(ctx, order.ID, respBody.Accrual); err != nil {
			slog.Error("failed to set accrual", slog.Any("error", err))
			return fmt.Errorf("failed to set accrual: %w", err)
		}
	case
		model.StatusInvalid.String(),
		model.StatusProcessing.String(),
		"REGISTERED":
		if err := c.repo.SetStatus(ctx, order.ID, model.StatusProcessing.String()); err != nil {
			slog.Error(
				"failed to set order status",
				slog.Any("error", err),
				slog.String("status", respBody.Status),
			)
			return fmt.Errorf("failed to set order status: %w", err)
		}
	default:
		slog.Error("unknown order status", slog.String("status", respBody.Status))
		return fmt.Errorf("unknown order status")
	}

	return nil
}

func (c *Collector) setRetryAfter(d time.Duration) {
	c.nextAllowed.Store(time.Now().Add(d).UnixNano())
}
