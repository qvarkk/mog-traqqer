package user

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/qvarkk/go-fiber-template/backend/internal/db/dbgen"
	"github.com/qvarkk/go-fiber-template/backend/internal/grpcserver"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100

	emailMaxLen     = 320
	displayNameMax  = 200
	uniqueViolation = "23505"
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Service owns the users business rules. It is the single place where input is
// validated and database errors are mapped onto gRPC statuses, so both the HTTP
// gateway and the gRPC service agree by construction.
type Service struct {
	queries dbgen.Querier
	log     *zap.Logger
}

func NewService(queries dbgen.Querier, log *zap.Logger) *Service {
	return &Service{queries: queries, log: log.Named("user.service")}
}

func (s *Service) Create(ctx context.Context, email, displayName string) (dbgen.User, error) {
	if err := validateEmail(email); err != nil {
		return dbgen.User{}, err
	}
	if len(displayName) > displayNameMax {
		return dbgen.User{}, fieldError("display_name", "validation.max", "this must be at most 200 characters long")
	}

	created, err := s.queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       email,
		DisplayName: displayName,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dbgen.User{}, grpcserver.Err(s.log, codes.AlreadyExists, "email_taken", "that email address is already registered", nil)
		}
		return dbgen.User{}, grpcserver.Err(s.log, codes.Internal, "user_create_failed", "could not create the user", err)
	}

	return created, nil
}

func (s *Service) Get(ctx context.Context, id string) (dbgen.User, error) {
	parsed, err := s.parseUUID(id)
	if err != nil {
		return dbgen.User{}, err
	}

	stored, err := s.queries.GetUser(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, grpcserver.Err(s.log, codes.NotFound, "user_not_found", "no user with that id", nil)
		}
		return dbgen.User{}, grpcserver.Err(s.log, codes.Internal, "user_lookup_failed", "could not load the user", err)
	}

	return stored, nil
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]dbgen.User, int32, int32, error) {
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit < 1 || limit > maxPageSize {
		return nil, 0, 0, grpcserver.Err(s.log, codes.InvalidArgument, "invalid_limit", "limit must be between 1 and 100", nil)
	}
	if offset < 0 {
		return nil, 0, 0, grpcserver.Err(s.log, codes.InvalidArgument, "invalid_offset", "offset must be zero or greater", nil)
	}

	stored, err := s.queries.ListUsers(ctx, dbgen.ListUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, 0, grpcserver.Err(s.log, codes.Internal, "user_list_failed", "could not list users", err)
	}

	return stored, limit, offset, nil
}

func (s *Service) Update(ctx context.Context, id, displayName string) (dbgen.User, error) {
	parsed, err := s.parseUUID(id)
	if err != nil {
		return dbgen.User{}, err
	}
	if displayName == "" {
		return dbgen.User{}, fieldError("display_name", "validation.required", "this field is required")
	}
	if len(displayName) > displayNameMax {
		return dbgen.User{}, fieldError("display_name", "validation.max", "this must be at most 200 characters long")
	}

	updated, err := s.queries.UpdateUserDisplayName(ctx, dbgen.UpdateUserDisplayNameParams{
		ID:          parsed,
		DisplayName: displayName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, grpcserver.Err(s.log, codes.NotFound, "user_not_found", "no user with that id", nil)
		}
		return dbgen.User{}, grpcserver.Err(s.log, codes.Internal, "user_update_failed", "could not update the user", err)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	parsed, err := s.parseUUID(id)
	if err != nil {
		return err
	}

	affected, err := s.queries.DeleteUser(ctx, parsed)
	if err != nil {
		return grpcserver.Err(s.log, codes.Internal, "user_delete_failed", "could not delete the user", err)
	}
	if affected == 0 {
		return grpcserver.Err(s.log, codes.NotFound, "user_not_found", "no user with that id", nil)
	}

	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return fieldError("email", "validation.required", "this field is required")
	}
	if len(email) > emailMaxLen || !emailPattern.MatchString(email) {
		return fieldError("email", "validation.email", "this must be a valid email address")
	}
	return nil
}

func fieldError(field, key, description string) error {
	return grpcserver.InvalidArgument("validation_failed", []grpcserver.FieldViolation{{
		Field:       field,
		Key:         key,
		Description: description,
	}})
}

func (s *Service) parseUUID(id string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		return pgtype.UUID{}, grpcserver.Err(s.log, codes.InvalidArgument, "invalid_id", "id must be a valid UUID", nil)
	}
	return parsed, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
