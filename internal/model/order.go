package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragpit/gophermart/internal/utils/luhn"
)

var (
	ErrOrderAlreadyAdded            = errors.New("order already added")
	ErrOrderAlreadyAddedByOtherUser = errors.New(
		"order already added by other user",
	)
	ErrBadOrderNumber = errors.New("bad order number format")
)

type OrderStatus int

const (
	StatusNew OrderStatus = iota
	StatusRegistered
	StatusProcessing
	StatusProcessed
	StatusInvalid
)

type OrdersRepository interface {
	GetOrdersByUserID(ctx context.Context, userID int) ([]Order, error)
	AddOrder(ctx context.Context, order *Order) error
}

type Order struct {
	ID         int
	UserID     int
	Number     string
	Status     OrderStatus
	Accrual    int
	UploadedAt time.Time
}

func NewOrder(userID int, num string) *Order {
	return &Order{
		UserID: userID,
		Number: num,
		Status: StatusNew,
	}
}

func ValidateNumber(number string) bool {
	return luhn.ValidateNumber(number)
}

func (s OrderStatus) String() string {
	switch s {
	case StatusNew:
		return "NEW"
	case StatusRegistered:
		return "REGISTERED"
	case StatusProcessing:
		return "PROCESSING"
	case StatusProcessed:
		return "PROCESSED"
	case StatusInvalid:
		return "INVALID"
	default:
		return "UNKNOWN"
	}
}

func (s OrderStatus) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *OrderStatus) UnmarshalText(text []byte) error {
	return s.fromString(string(text))
}

func (s *OrderStatus) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return s.fromString(v)
	case []byte:
		return s.fromString(string(v))
	default:
		return fmt.Errorf("unsupported type %T", src)
	}
}

func (s *OrderStatus) fromString(v string) error {
	switch v {
	case "NEW":
		*s = StatusNew
	case "REGISTERED":
		*s = StatusRegistered
	case "PROCESSING":
		*s = StatusProcessing
	case "PROCESSED":
		*s = StatusProcessed
	case "INVALID":
		*s = StatusInvalid
	default:
		return fmt.Errorf("unknown status %q", v)
	}
	return nil
}
