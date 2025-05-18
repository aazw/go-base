package postgres

import (
	"context"

	"github.com/aazw/go-base/pkg/cerrors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		return nil, cerrors.ErrDBOperation.New(
			cerrors.WithCause(err),
		)
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

	record, err := p.usersQueries.CreateUser(ctx, users.CreateUserParams{
		ID:    prototype.ID,
		Name:  prototype.Name,
		Email: prototype.Email,
	})
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505": // unique_violation
				return nil, cerrors.ErrDBDuplicate.New(
					cerrors.WithCause(err),
				)
			case "23503": // foreign_key_violation
				return nil, cerrors.ErrDBConstraint.New(
					cerrors.WithCause(err),
				)
			}
		}
		return nil, cerrors.ErrDBOperation.New(
			cerrors.WithCause(err),
		)
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
		if err == pgx.ErrNoRows {
			return nil, cerrors.ErrDBNotFound.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("record not found"),
			)
		}
		return nil, cerrors.ErrDBOperation.New(
			cerrors.WithCause(err),
		)
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
		if err == pgx.ErrNoRows {
			return nil, cerrors.ErrDBNotFound.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("record not found"),
			)
		}
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505": // unique_violation
				return nil, cerrors.ErrDBDuplicate.New(
					cerrors.WithCause(err),
				)
			case "23503": // foreign_key_violation
				return nil, cerrors.ErrDBConstraint.New(
					cerrors.WithCause(err),
				)
			}
		}
		return nil, cerrors.ErrDBOperation.New(
			cerrors.WithCause(err),
		)
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
		return cerrors.ErrDBOperation.New(
			cerrors.WithCause(err),
		)
	}
	if ret == 0 {
		return cerrors.ErrDBNotFound.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("record not found"),
		)
	}
	return nil
}
