package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	catalogv1 "github.com/qvarkk/mog-traqqer/backend/gen/catalog/v1"
	"github.com/qvarkk/mog-traqqer/backend/internal/authn"
	"github.com/qvarkk/mog-traqqer/backend/internal/db/dbgen"
)

// GRPCHandler adapts the service to the generated server interface.
type GRPCHandler struct {
	catalogv1.UnimplementedCatalogServiceServer

	service *Service
}

func NewGRPCHandler(service *Service) *GRPCHandler {
	return &GRPCHandler{service: service}
}

func (h *GRPCHandler) ListMuscleGroups(ctx context.Context, req *catalogv1.ListMuscleGroupsRequest) (*catalogv1.ListMuscleGroupsResponse, error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	groups, err := h.service.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]*catalogv1.MuscleGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, toProto(g))
	}
	return &catalogv1.ListMuscleGroupsResponse{MuscleGroups: out}, nil
}

func (h *GRPCHandler) CreateMuscleGroup(ctx context.Context, req *catalogv1.CreateMuscleGroupRequest) (*catalogv1.CreateMuscleGroupResponse, error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	created, err := h.service.Create(ctx, userID, req.GetName())
	if err != nil {
		return nil, err
	}
	return &catalogv1.CreateMuscleGroupResponse{MuscleGroup: toProto(created)}, nil
}

func (h *GRPCHandler) UpdateMuscleGroup(ctx context.Context, req *catalogv1.UpdateMuscleGroupRequest) (*catalogv1.UpdateMuscleGroupResponse, error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	updated, err := h.service.Update(ctx, userID, req.GetId(), req.GetName())
	if err != nil {
		return nil, err
	}
	return &catalogv1.UpdateMuscleGroupResponse{MuscleGroup: toProto(updated)}, nil
}

func (h *GRPCHandler) DeleteMuscleGroup(ctx context.Context, req *catalogv1.DeleteMuscleGroupRequest) (*catalogv1.DeleteMuscleGroupResponse, error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.service.Delete(ctx, userID, req.GetId()); err != nil {
		return nil, err
	}
	return &catalogv1.DeleteMuscleGroupResponse{}, nil
}

func requireUser(ctx context.Context) (pgtype.UUID, error) {
	userID, ok := authn.UserIDFromContext(ctx)
	if !ok {
		return pgtype.UUID{}, status.Error(codes.Unauthenticated, "missing credentials")
	}
	return userID, nil
}

func toProto(g dbgen.MuscleGroup) *catalogv1.MuscleGroup {
	return &catalogv1.MuscleGroup{
		Id:        g.ID.String(),
		Name:      g.Name,
		IsSystem:  !g.OwnerID.Valid,
		CreatedAt: timestamppb.New(g.CreatedAt.Time),
		UpdatedAt: timestamppb.New(g.UpdatedAt.Time),
	}
}
