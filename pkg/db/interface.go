package db

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazw/go-base/pkg/models"
)

type Handler interface {
	ListUsers(ctx context.Context, params models.ListUsersParams) ([]*models.User, error)
	CreateUser(ctx context.Context, prototype *models.UserPrototype) (*models.User, error)
	GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error)
	UpdateUser(ctx context.Context, userID uuid.UUID, prototype *models.UserPrototype) (*models.User, error)
	DeleteUSer(ctx context.Context, userID uuid.UUID) error
}
