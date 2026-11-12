// Package catalog owns reference data shared by every user: muscle groups and
// exercises. Muscle groups are implemented first; exercises land in the next
// step.
package catalog

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/qvarkk/mog-traqqer/backend/internal/db/dbgen"
	"github.com/qvarkk/mog-traqqer/backend/internal/grpcserver"
)

const nameMaxLen = 200

// Service owns the catalog business rules.
type Service struct {
	queries dbgen.Querier
	log     *zap.Logger
}

func NewService(queries dbgen.Querier, log *zap.Logger) *Service {
	return &Service{queries: queries, log: log.Named("catalog.service")}
}

func (s *Service) List(ctx context.Context, userID pgtype.UUID) ([]dbgen.MuscleGroup, error) {
	stored, err := s.queries.ListMuscleGroups(ctx, userID)
	if err != nil {
		return nil, grpcserver.Err(s.log, codes.Internal, "muscle_group_list_failed", "could not list muscle groups", err)
	}
	return stored, nil
}

func (s *Service) Create(ctx context.Context, userID pgtype.UUID, name string) (dbgen.MuscleGroup, error) {
	if err := validateName(name); err != nil {
		return dbgen.MuscleGroup{}, err
	}

	created, err := s.queries.CreateMuscleGroup(ctx, dbgen.CreateMuscleGroupParams{
		OwnerID: userID,
		Name:    name,
	})
	if err != nil {
		return dbgen.MuscleGroup{}, grpcserver.Err(s.log, codes.Internal, "muscle_group_create_failed", "could not create the muscle group", err)
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, userID pgtype.UUID, id, name string) (dbgen.MuscleGroup, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return dbgen.MuscleGroup{}, err
	}
	if err := validateName(name); err != nil {
		return dbgen.MuscleGroup{}, err
	}

	stored, err := s.queries.GetMuscleGroup(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.MuscleGroup{}, s.notFound()
		}
		return dbgen.MuscleGroup{}, grpcserver.Err(s.log, codes.Internal, "muscle_group_lookup_failed", "could not load the muscle group", err)
	}
	if err := requireOwner(stored.OwnerID, userID); err != nil {
		return dbgen.MuscleGroup{}, err
	}

	updated, err := s.queries.UpdateMuscleGroup(ctx, dbgen.UpdateMuscleGroupParams{ID: parsed, Name: name})
	if err != nil {
		return dbgen.MuscleGroup{}, grpcserver.Err(s.log, codes.Internal, "muscle_group_update_failed", "could not update the muscle group", err)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID pgtype.UUID, id string) error {
	parsed, err := parseUUID(id)
	if err != nil {
		return err
	}

	stored, err := s.queries.GetMuscleGroup(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.notFound()
		}
		return grpcserver.Err(s.log, codes.Internal, "muscle_group_lookup_failed", "could not load the muscle group", err)
	}
	if err := requireOwner(stored.OwnerID, userID); err != nil {
		return err
	}

	if _, err := s.queries.SoftDeleteMuscleGroup(ctx, parsed); err != nil {
		return grpcserver.Err(s.log, codes.Internal, "muscle_group_delete_failed", "could not delete the muscle group", err)
	}
	return nil
}

func (s *Service) notFound() error {
	return grpcserver.Err(s.log, codes.NotFound, "muscle_group_not_found", "no muscle group with that id", nil)
}

func validateName(name string) error {
	if name == "" {
		return fieldError("name", "validation.required", "this field is required")
	}
	if len(name) > nameMaxLen {
		return fieldError("name", "validation.max", "this must be at most 200 characters long")
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
		return pgtype.UUID{}, grpcserver.Err(nil, codes.InvalidArgument, "invalid_id", "id must be a valid UUID", nil)
	}
	return id, nil
}

// requireOwner rejects writes to system rows (owner NULL) and to rows owned by
// someone else.
func requireOwner(ownerID, userID pgtype.UUID) error {
	if !ownerID.Valid || ownerID.Bytes != userID.Bytes {
		return grpcserver.Err(nil, codes.PermissionDenied, "forbidden", "you do not own this muscle group", nil)
	}
	return nil
}
