package handlers

import (
	"context"

	"github.com/fragpit/gophermart/internal/api/middleware"
)

func UserIDFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(middleware.CtxUserIDKey)
	if v == nil {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}
