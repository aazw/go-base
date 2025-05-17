package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserPrototype
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

type UserPrototype struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type ListUsersParams struct {
}
