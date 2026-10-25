package gateway

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/qvarkk/mog-traqqer/backend/internal/config"
)

// Client is the gateway's handle on the users service.
type Client struct {
	health healthpb.HealthClient
	conn   *grpc.ClientConn
}

func NewClient(lc fx.Lifecycle, cfg *config.Config) (*Client, error) {
	// passthrough:/// hands the address straight to net.Dial. The dns resolver
	// prefers ::1 for "localhost", which hangs when the server binds IPv4 only.
	conn, err := grpc.NewClient("passthrough:///"+cfg.Users.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial users service at %s: %w", cfg.Users.Addr, err)
	}

	client := &Client{
		health: healthpb.NewHealthClient(conn),
		conn:   conn,
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return conn.Close()
		},
	})

	return client, nil
}

// Ping reports whether the users service is reachable and SERVING.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.health.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("users service reports %s", resp.GetStatus())
	}
	return nil
}
