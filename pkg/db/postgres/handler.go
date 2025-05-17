package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazw/go-base/pkg/db/postgres/users"
	"github.com/aazw/go-base/pkg/models"
)

type Handler struct {
	pgPool       *pgxpool.Pool
	usersQueries *users.Queries
}

func NewHandler(pgPool *pgxpool.Pool) (*Handler, error) {

	return &Handler{
		pgPool:       pgPool,
		usersQueries: users.New(pgPool),
	}, nil
}

func (p *Handler) ListUsers(ctx context.Context, params models.ListUsersParams) ([]*models.User, error) {

	records, err := p.usersQueries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	users := []*models.User{}
	for _, record := range records {
		users = append(users, &models.User{
			ID:        record.ID,
			Name:      record.Name,
			Email:     record.Email,
			CreatedAt: record.CreatedAt.Time,
			UpdatedAt: record.UpdatedAt.Time,
		})
	}
	return users, nil
}

func (p *Handler) CreateUser(ctx context.Context, prototype *models.UserPrototype) (*models.User, error) {

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	record, err := p.usersQueries.CreateUser(ctx, users.CreateUserParams{
		ID:    uuidV7,
		Name:  prototype.Name,
		Email: prototype.Email,
	})
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:        record.ID,
		Name:      record.Name,
		Email:     record.Email,
		CreatedAt: record.CreatedAt.Time,
		UpdatedAt: record.UpdatedAt.Time,
	}, nil
}

func (p *Handler) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {

	record, err := p.usersQueries.GetUser(ctx, userID)

	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:        record.ID,
		Name:      record.Name,
		Email:     record.Email,
		CreatedAt: record.CreatedAt.Time,
		UpdatedAt: record.UpdatedAt.Time,
	}, nil
}

func (p *Handler) UpdateUser(ctx context.Context, userID uuid.UUID, prototype *models.UserPrototype) (*models.User, error) {

	record, err := p.usersQueries.UpdateUser(ctx, users.UpdateUserParams{
		ID:    userID,
		Name:  prototype.Name,
		Email: prototype.Email,
	})
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:        record.ID,
		Name:      record.Name,
		Email:     record.Email,
		CreatedAt: record.CreatedAt.Time,
		UpdatedAt: record.UpdatedAt.Time,
	}, nil
}

func (p *Handler) DeleteUSer(ctx context.Context, userID uuid.UUID) error {

	ret, err := p.usersQueries.DeleteUser(ctx, userID)

	if err != nil {
		return err
	}
	if ret == 0 {
		return errors.New("")
	}

	return nil
}
