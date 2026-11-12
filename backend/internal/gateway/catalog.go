package gateway

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/grpc/metadata"

	catalogv1 "github.com/qvarkk/mog-traqqer/backend/gen/catalog/v1"
	"github.com/qvarkk/mog-traqqer/backend/internal/server"
)

// authContext forwards the caller's Authorization header to the upstream
// service, where the auth interceptor validates it.
func authContext(c fiber.Ctx) context.Context {
	ctx := c.Context()
	if token := c.Get("Authorization"); token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", token)
	}
	return ctx
}

func (h *Handler) ListMuscleGroups(c fiber.Ctx) error {
	resp, err := h.catalog.ListMuscleGroups(authContext(c), &catalogv1.ListMuscleGroupsRequest{})
	if err != nil {
		return fromGRPC(c, err)
	}

	data := make([]map[string]any, 0, len(resp.GetMuscleGroups()))
	for _, g := range resp.GetMuscleGroups() {
		data = append(data, muscleGroupJSON(g))
	}
	return c.JSON(fiber.Map{"data": data})
}

func (h *Handler) CreateMuscleGroup(c fiber.Ctx) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.catalog.CreateMuscleGroup(authContext(c), &catalogv1.CreateMuscleGroupRequest{Name: req.Name})
	if err != nil {
		return fromGRPC(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(muscleGroupJSON(resp.GetMuscleGroup()))
}

func (h *Handler) UpdateMuscleGroup(c fiber.Ctx) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.catalog.UpdateMuscleGroup(authContext(c), &catalogv1.UpdateMuscleGroupRequest{
		Id:   c.Params("id"),
		Name: req.Name,
	})
	if err != nil {
		return fromGRPC(c, err)
	}
	return c.JSON(muscleGroupJSON(resp.GetMuscleGroup()))
}

func (h *Handler) DeleteMuscleGroup(c fiber.Ctx) error {
	if _, err := h.catalog.DeleteMuscleGroup(authContext(c), &catalogv1.DeleteMuscleGroupRequest{Id: c.Params("id")}); err != nil {
		return fromGRPC(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func muscleGroupJSON(g *catalogv1.MuscleGroup) map[string]any {
	return map[string]any{
		"id":         g.GetId(),
		"name":       g.GetName(),
		"is_system":  g.GetIsSystem(),
		"created_at": formatTime(g.GetCreatedAt()),
		"updated_at": formatTime(g.GetUpdatedAt()),
	}
}
