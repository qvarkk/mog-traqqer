package user

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/qvarkk/go-fiber-template/backend/gen/user/v1"
	"github.com/qvarkk/go-fiber-template/backend/internal/db/dbgen"
)

// GRPCHandler adapts the service to the generated server interface. It owns no
// business rules; it only parses requests, calls the service and builds the
// response message.
type GRPCHandler struct {
	// Keeping this embedded means a newly added RPC answers Unimplemented
	// instead of breaking the build.
	userv1.UnimplementedUserServiceServer

	service *Service
}

func NewGRPCHandler(service *Service) *GRPCHandler {
	return &GRPCHandler{service: service}
}

func (h *GRPCHandler) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	created, err := h.service.Create(ctx, req.GetEmail(), req.GetDisplayName())
	if err != nil {
		return nil, err
	}
	return &userv1.CreateUserResponse{User: toProto(created)}, nil
}

func (h *GRPCHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	stored, err := h.service.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &userv1.GetUserResponse{User: toProto(stored)}, nil
}

func (h *GRPCHandler) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	stored, limit, offset, err := h.service.List(ctx, req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, err
	}

	users := make([]*userv1.User, 0, len(stored))
	for _, row := range stored {
		users = append(users, toProto(row))
	}

	return &userv1.ListUsersResponse{
		Users:  users,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (h *GRPCHandler) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	updated, err := h.service.Update(ctx, req.GetId(), req.GetDisplayName())
	if err != nil {
		return nil, err
	}
	return &userv1.UpdateUserResponse{User: toProto(updated)}, nil
}

func (h *GRPCHandler) DeleteUser(ctx context.Context, req *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	if err := h.service.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &userv1.DeleteUserResponse{}, nil
}

func toProto(stored dbgen.User) *userv1.User {
	return &userv1.User{
		Id:          stored.ID.String(),
		Email:       stored.Email,
		DisplayName: stored.DisplayName,
		CreatedAt:   timestamppb.New(stored.CreatedAt.Time),
		UpdatedAt:   timestamppb.New(stored.UpdatedAt.Time),
	}
}
