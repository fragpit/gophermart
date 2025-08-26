package model

import "context"

type UserRepository interface {
	Create(ctx context.Context, login, passwordHash string) error
	GetByLogin(ctx context.Context, login string) (*User, error)
}

type User struct {
	ID           int
	Login        string
	PasswordHash string
}

func NewUser(login string) *User {
	return &User{}
}
