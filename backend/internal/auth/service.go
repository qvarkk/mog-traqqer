// Package auth owns identity and credentials: registration, login, and
// access/refresh token issuance and revocation.
package auth

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/qvarkk/mog-traqqer/backend/internal/authn"
	"github.com/qvarkk/mog-traqqer/backend/internal/config"
	"github.com/qvarkk/mog-traqqer/backend/internal/db/dbgen"
	"github.com/qvarkk/mog-traqqer/backend/internal/grpcserver"
)

const (
	passwordMinLen  = 8
	passwordMaxLen  = 72
	uniqueViolation = "23505"
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Service owns the auth business rules. It is the single place where input is
// validated, passwords are hashed, tokens are issued and database errors are
// mapped onto gRPC statuses.
type Service struct {
	queries dbgen.Querier
	log     *zap.Logger
	cfg     config.AuthConfig
}

func NewService(queries dbgen.Querier, log *zap.Logger, cfg *config.Config) (*Service, error) {
	if cfg.Auth.JWTSecret == "" {
		return nil, errors.New("AUTH_JWT_SECRET is empty; set it before starting the auth service")
	}
	return &Service{queries: queries, log: log.Named("auth.service"), cfg: cfg.Auth}, nil
}

func (s *Service) Register(ctx context.Context, email, password, deviceInfo string) (dbgen.User, string, string, error) {
	if err := validateEmail(email); err != nil {
		return dbgen.User{}, "", "", err
	}
	if err := validatePassword(password); err != nil {
		return dbgen.User{}, "", "", err
	}

	hash, err := hashPassword(password, s.argon2())
	if err != nil {
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "password_hash_failed", "could not hash the password", err)
	}

	userID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	created, err := s.queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           userID,
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.AlreadyExists, "email_taken", "that email address is already registered", nil)
		}
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "user_create_failed", "could not create the user", err)
	}

	access, refresh, err := s.issueTokens(ctx, created.ID, deviceInfo)
	if err != nil {
		return dbgen.User{}, "", "", err
	}
	return created, access, refresh, nil
}

func (s *Service) Login(ctx context.Context, email, password, deviceInfo string) (dbgen.User, string, string, error) {
	if err := validateEmail(email); err != nil {
		return dbgen.User{}, "", "", err
	}
	if password == "" {
		return dbgen.User{}, "", "", fieldError("password", "validation.required", "this field is required")
	}

	stored, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, "", "", s.invalidCredentials()
		}
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "user_lookup_failed", "could not load the user", err)
	}

	ok, err := verifyPassword(password, stored.PasswordHash)
	if err != nil {
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "password_verify_failed", "could not verify the password", err)
	}
	if !ok {
		return dbgen.User{}, "", "", s.invalidCredentials()
	}

	access, refresh, err := s.issueTokens(ctx, stored.ID, deviceInfo)
	if err != nil {
		return dbgen.User{}, "", "", err
	}
	return stored, access, refresh, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (dbgen.User, string, string, error) {
	jti, err := parseUUID(refreshToken)
	if err != nil {
		return dbgen.User{}, "", "", s.invalidToken()
	}

	stored, err := s.queries.GetRefreshToken(ctx, jti)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, "", "", s.invalidToken()
		}
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "token_lookup_failed", "could not load the refresh token", err)
	}

	if stored.IsRevoked {
		// A rotated token was reused: assume theft and revoke the whole chain.
		if _, err := s.queries.RevokeUserTokens(ctx, stored.UserID); err != nil {
			s.log.Error("could not revoke user tokens after reuse", zap.Error(err))
		}
		return dbgen.User{}, "", "", s.invalidToken()
	}

	if stored.ExpiresAt.Time.Before(time.Now()) {
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Unauthenticated, "token_expired", "the refresh token has expired", nil)
	}

	if _, err := s.queries.RevokeRefreshToken(ctx, jti); err != nil {
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "token_rotate_failed", "could not rotate the refresh token", err)
	}

	user, err := s.queries.GetUser(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, "", "", s.invalidToken()
		}
		return dbgen.User{}, "", "", grpcserver.Err(s.log, codes.Internal, "user_lookup_failed", "could not load the user", err)
	}

	access, refresh, err := s.issueTokens(ctx, user.ID, "")
	if err != nil {
		return dbgen.User{}, "", "", err
	}
	return user, access, refresh, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	jti, err := parseUUID(refreshToken)
	if err != nil {
		return nil // idempotent: an unparseable token is already effectively logged out
	}

	if _, err := s.queries.RevokeRefreshToken(ctx, jti); err != nil {
		return grpcserver.Err(s.log, codes.Internal, "token_revoke_failed", "could not revoke the refresh token", err)
	}
	return nil
}

func (s *Service) GetMe(ctx context.Context, userID pgtype.UUID) (dbgen.User, error) {
	stored, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.User{}, s.invalidToken()
		}
		return dbgen.User{}, grpcserver.Err(s.log, codes.Internal, "user_lookup_failed", "could not load the user", err)
	}
	return stored, nil
}

func (s *Service) issueTokens(ctx context.Context, userID pgtype.UUID, deviceInfo string) (string, string, error) {
	access, err := authn.Sign(s.cfg.JWTSecret, userID, s.cfg.AccessTTL)
	if err != nil {
		return "", "", grpcserver.Err(s.log, codes.Internal, "token_sign_failed", "could not sign the access token", err)
	}

	jti := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	expires := pgtype.Timestamptz{Time: time.Now().Add(s.cfg.RefreshTTL), Valid: true}

	if err := s.queries.InsertRefreshToken(ctx, dbgen.InsertRefreshTokenParams{
		Jti:       jti,
		UserID:    userID,
		Column3:   deviceInfo,
		ExpiresAt: expires,
	}); err != nil {
		return "", "", grpcserver.Err(s.log, codes.Internal, "token_store_failed", "could not store the refresh token", err)
	}

	return access, jti.String(), nil
}

func (s *Service) argon2() argon2Params {
	return argon2Params{
		memory:      s.cfg.Argon2Memory,
		iterations:  s.cfg.Argon2Iterations,
		parallelism: s.cfg.Argon2Parallelism,
	}
}

func (s *Service) invalidCredentials() error {
	return grpcserver.Err(s.log, codes.Unauthenticated, "invalid_credentials", "invalid email or password", nil)
}

func (s *Service) invalidToken() error {
	return grpcserver.Err(s.log, codes.Unauthenticated, "invalid_token", "the token is invalid", nil)
}

func validateEmail(email string) error {
	if email == "" {
		return fieldError("email", "validation.required", "this field is required")
	}
	if len(email) > 320 || !emailPattern.MatchString(email) {
		return fieldError("email", "validation.email", "this must be a valid email address")
	}
	return nil
}

func validatePassword(password string) error {
	if password == "" {
		return fieldError("password", "validation.required", "this field is required")
	}
	if len(password) < passwordMinLen {
		return fieldError("password", "validation.password_too_short", "the password must be at least 8 characters long")
	}
	if len(password) > passwordMaxLen {
		return fieldError("password", "validation.password_too_long", "the password must be at most 72 characters long")
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

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
