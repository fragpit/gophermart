package orders

import (
	"context"

	"github.com/fragpit/gophermart/internal/api/handlers"
	"github.com/fragpit/gophermart/internal/model"
)

var _ handlers.OrdersService = (*OrdersService)(nil)

type OrdersService struct {
	repo model.OrdersRepository
}

func NewOrdersService(repo model.OrdersRepository) *OrdersService {
	return &OrdersService{
		repo: repo,
	}
}

func (o *OrdersService) GetOrdersByUser(
	ctx context.Context,
	userID int,
) ([]model.Order, error) {
	return o.repo.GetOrdersByUserID(ctx, userID)
}

func (o *OrdersService) AddOrder(
	ctx context.Context,
	userID int,
	orderNumber string,
) error {
	order := model.NewOrder(userID, orderNumber)

	return o.repo.AddOrder(ctx, order)
}
