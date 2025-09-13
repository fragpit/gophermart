package postgresql

import (
	"context"
	"fmt"

	"github.com/fragpit/gophermart/internal/model"
	"github.com/jackc/pgx/v5"
)

var _ model.UsersRepository = (*UsersRepo)(nil)

type UsersRepo struct {
	baseRepo
}

func (r *UsersRepo) Create(
	ctx context.Context,
	user *model.User,
) (*model.User, error) {
	q := `
		INSERT INTO users (login, password_hash)
		VALUES (@login, @password_hash)
		RETURNING id;
	`

	args := pgx.NamedArgs{
		"login":         user.Login,
		"password_hash": user.PasswordHash,
	}

	var id int32
	row := r.db.QueryRow(ctx, q, args)
	if err := row.Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	user.ID = int(id)

	return user, nil
}

func (r *UsersRepo) GetByLogin(
	ctx context.Context,
	login string,
) (*model.User, error) {
	q := `
		SELECT id, login, password_hash
		FROM users
		WHERE login = $1
	`

	var (
		userID    int
		userLogin string
		userPHash string
	)
	row := r.db.QueryRow(ctx, q, login)
	if err := row.Scan(&userID, &userLogin, &userPHash); err != nil {
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	u := &model.User{
		ID:           userID,
		Login:        userLogin,
		PasswordHash: userPHash,
	}

	return u, nil
}
