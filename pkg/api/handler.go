package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oapi-codegen/runtime/types"

	"goapp/pkg/api/openapi"
	"goapp/pkg/db/users"
)

// StrictServerInterface の実装用
type StrictServerImpl struct {
	dbPool    *pgxpool.Pool
	redisPool *redis.Pool
	sm        *scs.SessionManager
}

func NewStrictServerImpl(dbPool *pgxpool.Pool, redisPool *redis.Pool, sm *scs.SessionManager) openapi.StrictServerInterface {

	return &StrictServerImpl{
		dbPool:    dbPool,
		redisPool: redisPool,
		sm:        sm,
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
		return openapi.GetHealthReadiness503JSONResponse{
			Status: openapi.Unavailable,
		}, nil
	}

	// ping to redis
	conn := p.redisPool.Get()
	defer conn.Close()
	_, err := redis.String(conn.Do("PING"))
	if err != nil {
		return openapi.GetHealthReadiness503JSONResponse{
			Status: openapi.Unavailable,
		}, nil
	}

	return openapi.GetHealthReadiness200JSONResponse{
		Status: openapi.Available,
	}, nil
}

// List all users
// (GET /users)
func (p *StrictServerImpl) ListUsers(ctx context.Context, request openapi.ListUsersRequestObject) (openapi.ListUsersResponseObject, error) {

	items, err := users.New(p.dbPool).ListUsers(ctx)
	if err != nil {
		return openapi.ListUsers500JSONResponse{
			Code:    1,
			Message: "internal server error",
		}, nil
	}

	var retItems []openapi.User
	for _, item := range items {
		retItems = append(retItems, openapi.User{
			Id:    item.ID,
			Name:  item.Name,
			Email: types.Email(item.Email),
		})
	}

	return openapi.ListUsers200JSONResponse{
		Users: retItems,
	}, nil
}

// Create a new user
// (POST /users)
func (p *StrictServerImpl) CreateUser(ctx context.Context, request openapi.CreateUserRequestObject) (openapi.CreateUserResponseObject, error) {

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return openapi.CreateUser500JSONResponse{
			Code:    1,
			Message: "internal server error",
		}, nil
	}

	user, err := users.New(p.dbPool).CreateUser(ctx, users.CreateUserParams{
		ID:    uuidV7,
		Name:  request.Body.Name,
		Email: strings.ToLower(string(request.Body.Email)),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code { // ← 文字列か定数で比較
			case pgerrcode.UniqueViolation: // 23505
				fallthrough
			case pgerrcode.ForeignKeyViolation: // 23503
				return openapi.CreateUser400JSONResponse{
					Code:    1,
					Message: "bad request error",
				}, nil
			default:
				// そのほかの制約違反
				return openapi.CreateUser500JSONResponse{
					Code:    1,
					Message: "internal server error",
				}, nil
			}
		}

		// *pgconn.PgError 以外のエラー
		return openapi.CreateUser500JSONResponse{
			Code:    1,
			Message: "internal server error",
		}, nil
	}

	return openapi.CreateUser201JSONResponse{
		User: openapi.User{
			Id:    user.ID,
			Name:  user.Name,
			Email: types.Email(user.Email),
		},
	}, nil
}

// Get a user by ID
// (GET /users/{user_id})
func (p *StrictServerImpl) GetUserById(ctx context.Context, request openapi.GetUserByIdRequestObject) (openapi.GetUserByIdResponseObject, error) {

	user, err := users.New(p.dbPool).GetUser(ctx, request.UserId)
	switch {
	case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
		return openapi.GetUserById404JSONResponse{
			Code:    2,
			Message: "resource not founc",
		}, nil
	case err != nil:
		return openapi.GetUserById500JSONResponse{
			Code:    1,
			Message: "internal server error",
		}, nil
	default:
		// 正常
		return openapi.GetUserById200JSONResponse{
			User: openapi.User{
				Id:    user.ID,
				Name:  user.Name,
				Email: types.Email(user.Email),
			},
		}, nil
	}
}

// Update a user by ID
// (PATCH /users/{user_id})
func (p *StrictServerImpl) UpdateUserById(ctx context.Context, request openapi.UpdateUserByIdRequestObject) (openapi.UpdateUserByIdResponseObject, error) {

	user, err := users.New(p.dbPool).UpdateUser(ctx, users.UpdateUserParams{
		Name:  request.Body.Name,
		Email: strings.ToLower(string(request.Body.Email)),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
		return openapi.UpdateUserById404JSONResponse{
			Code:    2,
			Message: "resource not founc",
		}, nil
	case err != nil:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code { // ← 文字列か定数で比較
			case pgerrcode.UniqueViolation: // 23505
				fallthrough
			case pgerrcode.ForeignKeyViolation: // 23503
				return openapi.UpdateUserById400JSONResponse{
					Code:    1,
					Message: "bad request error",
				}, nil
			default:
				// そのほかの制約違反
				return openapi.UpdateUserById500JSONResponse{
					Code:    1,
					Message: "internal server error",
				}, nil
			}
		}

		return openapi.UpdateUserById500JSONResponse{
			Code:    1,
			Message: "internal server error",
		}, nil
	default:
		// 正常
		return openapi.UpdateUserById200JSONResponse{
			User: openapi.User{
				Id:    user.ID,
				Name:  user.Name,
				Email: types.Email(user.Email),
			},
		}, nil
	}
}

// Delete a user by ID
// (DELETE /users/{user_id})
func (p *StrictServerImpl) DeleteUserById(ctx context.Context, request openapi.DeleteUserByIdRequestObject) (openapi.DeleteUserByIdResponseObject, error) {

	ret, err := users.New(p.dbPool).DeleteUser(ctx, request.UserId)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows):
			return openapi.DeleteUserById404JSONResponse{
				Code:    2,
				Message: "resource not founc",
			}, nil
		default:
			return openapi.DeleteUserById500JSONResponse{
				Code:    1,
				Message: "internal server error",
			}, nil
		}
	}
	if ret == 0 {
		return openapi.DeleteUserById404JSONResponse{
			Code:    1,
			Message: "resource not founc",
		}, nil
	}
	return openapi.DeleteUserById204Response{}, nil
}
