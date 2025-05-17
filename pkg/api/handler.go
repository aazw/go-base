// pkg/api/handler.go
package api

import (
	"context"

	"github.com/aazw/go-base/pkg/db"
	"github.com/aazw/go-base/pkg/models"
	"github.com/google/uuid"
)

type Handler struct {
	dbHandler db.Handler
}

func NewHandler(dbHandler db.Handler) (*Handler, error) {

	return &Handler{
		dbHandler: dbHandler,
	}, nil
}

func (p *Handler) ListUsers(ctx context.Context, params models.ListUsersParams) ([]*models.User, error) {

	return p.dbHandler.ListUsers(ctx, params)
}

func (p *Handler) CreateUser(ctx context.Context, prototype *models.UserPrototype) (*models.User, error) {

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	prototype.ID = uuidV7

	return p.dbHandler.CreateUser(ctx, prototype)
}

func (p *Handler) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {

	return p.dbHandler.GetUser(ctx, userID)
}

func (p *Handler) UpdateUser(ctx context.Context, userID uuid.UUID, prototype *models.UserPrototype) (*models.User, error) {

	prototype.ID = userID
	return p.dbHandler.UpdateUser(ctx, userID, prototype)
}

func (p *Handler) DeleteUser(ctx context.Context, userID uuid.UUID) (int, error) {

	return 1, p.dbHandler.DeleteUSer(ctx, userID)
}
