package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/qvarkk/mog-traqqer/backend/gen/auth/v1"
	"github.com/qvarkk/mog-traqqer/backend/internal/authn"
	"github.com/qvarkk/mog-traqqer/backend/internal/db/dbgen"
)

// GRPCHandler adapts the service to the generated server interface. It owns no
// business rules.
type GRPCHandler struct {
	authv1.UnimplementedAuthServiceServer

	service *Service
}

func NewGRPCHandler(service *Service) *GRPCHandler {
	return &GRPCHandler{service: service}
}

func (h *GRPCHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	user, access, refresh, err := h.service.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetDeviceInfo())
	if err != nil {
		return nil, err
	}
	return &authv1.RegisterResponse{AccessToken: access, RefreshToken: refresh, User: toProto(user)}, nil
}

func (h *GRPCHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	user, access, refresh, err := h.service.Login(ctx, req.GetEmail(), req.GetPassword(), req.GetDeviceInfo())
	if err != nil {
		return nil, err
	}
	return &authv1.LoginResponse{AccessToken: access, RefreshToken: refresh, User: toProto(user)}, nil
}

func (h *GRPCHandler) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	user, access, refresh, err := h.service.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &authv1.RefreshResponse{AccessToken: access, RefreshToken: refresh, User: toProto(user)}, nil
}

func (h *GRPCHandler) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := h.service.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, err
	}
	return &authv1.LogoutResponse{}, nil
}

func (h *GRPCHandler) GetMe(ctx context.Context, req *authv1.GetMeRequest) (*authv1.GetMeResponse, error) {
	userID, ok := authn.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}

	user, err := h.service.GetMe(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &authv1.GetMeResponse{User: toProto(user)}, nil
}

func toProto(u dbgen.User) *authv1.User {
	return &authv1.User{
		Id:        u.ID.String(),
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt.Time),
		UpdatedAt: timestamppb.New(u.UpdatedAt.Time),
	}
}
