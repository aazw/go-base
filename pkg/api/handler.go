// pkg/api/handler.go
package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazw/go-base/pkg/api/openapi"
	"github.com/aazw/go-base/pkg/cerrors"
	"github.com/aazw/go-base/pkg/models"
	"github.com/aazw/go-base/pkg/operations"
)

// StrictServerInterface の実装用
type StrictServerImpl struct {
	opsHandler *operations.Handler
	dbPool     *pgxpool.Pool
	redisPool  *redis.Pool
	sm         *scs.SessionManager
}

func NewStrictServerImpl(opsHandler *operations.Handler, dbPool *pgxpool.Pool, redisPool *redis.Pool, sm *scs.SessionManager) openapi.StrictServerInterface {

	return &StrictServerImpl{
		opsHandler: opsHandler,
		dbPool:     dbPool,
		redisPool:  redisPool,
		sm:         sm,
	}
}

// StrictServerInterfaceの実装

// Liveness チェック
// (GET /health/liveness)
func (p *StrictServerImpl) GetHealthLiveness(ctx context.Context, request openapi.GetHealthLivenessRequestObject) (openapi.GetHealthLivenessResponseObject, error) {

	return openapi.GetHealthLiveness200JSONResponse{
		Status: openapi.Available,
	}, nil
}

// Readiness チェック
// (GET /health/readiness)
func (p *StrictServerImpl) GetHealthReadiness(ctx context.Context, request openapi.GetHealthReadinessRequestObject) (openapi.GetHealthReadinessResponseObject, error) {

	// ping to relational database
	if err := p.dbPool.Ping(ctx); err != nil {
		cerr := cerrors.ErrServiceUnavailable.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("database is not available"),
		)
		return openapi.GetHealthReadiness503JSONResponse{
			Status: openapi.Unavailable,
		}, cerr
	}

	// ping to redis
	conn := p.redisPool.Get()
	defer conn.Close()
	_, err := redis.String(conn.Do("PING"))
	if err != nil {
		cerr := cerrors.ErrServiceUnavailable.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("redis is not available"),
		)
		return openapi.GetHealthReadiness503JSONResponse{
			Status: openapi.Unavailable,
		}, cerr
	}

	return openapi.GetHealthReadiness200JSONResponse{
		Status: openapi.Available,
	}, nil
}

// List all users
// (GET /users)
func (p *StrictServerImpl) ListUsers(ctx context.Context, request openapi.ListUsersRequestObject) (openapi.ListUsersResponseObject, error) {

	items, err := p.opsHandler.ListUsers(ctx, models.ListUsersParams{})
	if err != nil {
		cerr := cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to list users"),
		)
		return openapi.ListUsers500JSONResponse{
			Type:   stringPointer("/internal_server_error"),
			Title:  stringPointer(http.StatusText(500)),
			Status: intPointer(500),
		}, cerr
	}

	var retItems []openapi.User
	for _, item := range items {
		retItems = append(retItems, openapi.User{
			Id:    item.ID,
			Name:  item.Name,
			Email: item.Email,
		})
	}

	return openapi.ListUsers200JSONResponse{
		Users: retItems,
	}, nil
}

// Create a new user
// (POST /users)
func (p *StrictServerImpl) CreateUser(ctx context.Context, request openapi.CreateUserRequestObject) (openapi.CreateUserResponseObject, error) {

	user, err := p.opsHandler.CreateUser(ctx, &models.UserPrototype{
		Name:  request.Body.Name,
		Email: strings.ToLower(string(request.Body.Email)),
	})
	if err != nil {
		var cerr error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				cerr = cerrors.ErrDBDuplicate.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("user already exists"),
				)
				return openapi.CreateUser400JSONResponse{
					Type:   stringPointer("/bad_request"),
					Title:  stringPointer(http.StatusText(400)),
					Status: intPointer(400),
				}, cerr
			case pgerrcode.ForeignKeyViolation:
				cerr = cerrors.ErrDBConstraint.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("foreign key constraint violation"),
				)
				return openapi.CreateUser400JSONResponse{
					Type:   stringPointer("/bad_request"),
					Title:  stringPointer(http.StatusText(400)),
					Status: intPointer(400),
				}, cerr
			default:
				cerr = cerrors.ErrSystemInternal.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("database error"),
				)
				return openapi.CreateUser500JSONResponse{
					Type:   stringPointer("/internal_server_error"),
					Title:  stringPointer(http.StatusText(500)),
					Status: intPointer(500),
				}, cerr
			}
		}

		cerr = cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to create user"),
		)
		return openapi.CreateUser500JSONResponse{
			Type:   stringPointer("/internal_server_error"),
			Title:  stringPointer(http.StatusText(500)),
			Status: intPointer(500),
		}, cerr
	}

	return openapi.CreateUser201JSONResponse{
		User: openapi.User{
			Id:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		},
	}, nil
}

// Get a user by ID
// (GET /users/{user_id})
func (p *StrictServerImpl) GetUserById(ctx context.Context, request openapi.GetUserByIdRequestObject) (openapi.GetUserByIdResponseObject, error) {

	user, err := p.opsHandler.GetUser(ctx, uuid.MustParse(request.UserId))
	switch {
	case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
		cerr := cerrors.ErrDBNotFound.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("user not found"),
		)
		return openapi.GetUserById404JSONResponse{
			Type:   stringPointer("/resource_not_found"),
			Title:  stringPointer(http.StatusText(404)),
			Status: intPointer(404),
		}, cerr
	case err != nil:
		cerr := cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to get user"),
		)
		return openapi.GetUserById500JSONResponse{
			Type:   stringPointer("/internal_server_error"),
			Title:  stringPointer(http.StatusText(500)),
			Status: intPointer(500),
		}, cerr
	default:
		// 正常
		return openapi.GetUserById200JSONResponse{
			User: openapi.User{
				Id:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
		}, nil
	}
}

// Update a user by ID
// (PATCH /users/{user_id})
func (p *StrictServerImpl) UpdateUserById(ctx context.Context, request openapi.UpdateUserByIdRequestObject) (openapi.UpdateUserByIdResponseObject, error) {

	user, err := p.opsHandler.UpdateUser(ctx, uuid.MustParse(request.UserId), &models.UserPrototype{
		Name:  request.Body.Name,
		Email: string(request.Body.Email),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
		cerr := cerrors.ErrDBNotFound.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("user not found"),
		)
		return openapi.UpdateUserById404JSONResponse{
			Type:   stringPointer("/resource_not_found"),
			Title:  stringPointer(http.StatusText(404)),
			Status: intPointer(404),
		}, cerr
	case err != nil:
		var cerr error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				cerr = cerrors.ErrDBDuplicate.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("user already exists"),
				)
				return openapi.UpdateUserById400JSONResponse{
					Type:   stringPointer("/bad_request"),
					Title:  stringPointer(http.StatusText(400)),
					Status: intPointer(400),
				}, cerr
			case pgerrcode.ForeignKeyViolation:
				cerr = cerrors.ErrDBConstraint.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("foreign key constraint violation"),
				)
				return openapi.UpdateUserById400JSONResponse{
					Type:   stringPointer("/bad_request"),
					Title:  stringPointer(http.StatusText(400)),
					Status: intPointer(400),
				}, cerr
			default:
				cerr = cerrors.ErrSystemInternal.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("database error"),
				)
				return openapi.UpdateUserById500JSONResponse{
					Type:   stringPointer("/internal_server_error"),
					Title:  stringPointer(http.StatusText(500)),
					Status: intPointer(500),
				}, cerr
			}
		}

		cerr = cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to update user"),
		)
		return openapi.UpdateUserById500JSONResponse{
			Type:   stringPointer("/internal_server_error"),
			Title:  stringPointer(http.StatusText(500)),
			Status: intPointer(500),
		}, cerr
	default:
		// 正常
		return openapi.UpdateUserById200JSONResponse{
			User: openapi.User{
				Id:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
		}, nil
	}
}

// Delete a user by ID
// (DELETE /users/{user_id})
func (p *StrictServerImpl) DeleteUserById(ctx context.Context, request openapi.DeleteUserByIdRequestObject) (openapi.DeleteUserByIdResponseObject, error) {

	ret, err := p.opsHandler.DeleteUser(ctx, uuid.MustParse(request.UserId))
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
			cerr := cerrors.ErrDBNotFound.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("user not found"),
			)
			return openapi.DeleteUserById404JSONResponse{
				Type:   stringPointer("/resource_not_found"),
				Title:  stringPointer(http.StatusText(404)),
				Status: intPointer(404),
			}, cerr
		default:
			cerr := cerrors.ErrSystemInternal.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to delete user"),
			)
			return openapi.DeleteUserById500JSONResponse{
				Type:   stringPointer("/internal_server_error"),
				Title:  stringPointer(http.StatusText(500)),
				Status: intPointer(500),
			}, cerr
		}
	}
	if ret == 0 {
		cerr := cerrors.ErrDBNotFound.New(
			cerrors.WithMessage("user not found"),
		)
		return openapi.DeleteUserById404JSONResponse{
			Type:   stringPointer("/resource_not_found"),
			Title:  stringPointer(http.StatusText(404)),
			Status: intPointer(404),
		}, cerr
	}
	return openapi.DeleteUserById204Response{}, nil
}
