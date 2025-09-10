package model

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fragpit/gophermart/internal/utils/luhn"
)

var (
	ErrOrderAlreadyExist            = errors.New("order already exist")
	ErrOrderAlreadyAddedByOtherUser = errors.New(
		"order already added by other user",
	)
	ErrBadOrderNumber = errors.New("bad order number format")
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
	Accrual    Kopek
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

type OrderStatus int

const (
	StatusNew OrderStatus = iota
	StatusProcessing
	StatusProcessed
	StatusInvalid
)

func (s OrderStatus) String() string {
	switch s {
	case StatusNew:
		return "NEW"
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

type Kopek int

func (k Kopek) MarshalJSON() ([]byte, error) {
	v := int(k)

	intPart := v / 100
	frac := v % 100

	// удовлетворим всем требованиям спецификации docs/SPECIFICATION.md
	// т.к. требований нет, просто визуальное соответствие.
	switch {
	case frac == 0:
		return []byte(fmt.Sprintf("%d", intPart)), nil
	case frac%10 == 0:
		return []byte(fmt.Sprintf("%d.%d", intPart, frac/10)), nil
	default:
		return []byte(fmt.Sprintf("%d.%02d", intPart, frac)), nil
	}
}

func (k *Kopek) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "" {
		*k = 0
		return nil
	}

	var result int
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) != 2 {
			return fmt.Errorf("invalid accrual value %s", s)
		}
		intPart, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid accrual value %s: %w", s, err)
		}
		fracPart := parts[1]
		if len(fracPart) == 1 {
			fracPart += "0"
		}
		if len(fracPart) != 2 {
			return fmt.Errorf("invalid accrual value %s", s)
		}
		frac, err := strconv.Atoi(fracPart)
		if err != nil {
			return fmt.Errorf("invalid accrual value %s: %w", s, err)
		}
		result = intPart*100 + frac
	} else {
		intPart, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid accrual value %s: %w", s, err)
		}
		result = intPart * 100
	}

	*k = Kopek(result)
	return nil
}
