package client

import (
	"context"
	"errors"
	"fmt"
	pb "pastebin/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.PasteServiceClient
}

func NewGRPCClient(address string) (*GRPCClient, error) {
	// Для локального стенда используем соединение без TLS.
	// Встроенные retries отключаем: повторами управляет doWithRetry.
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if err != nil {
		return nil, fmt.Errorf("create gRPC client: %w", err)
	}

	return &GRPCClient{conn: conn, client: pb.NewPasteServiceClient(conn)}, nil
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

func (c *GRPCClient) CreatePaste(ctx context.Context, input CreatePasteRequest, idempotencyKey string) (*CreatePasteResponse, error) {
	if idempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	// В Protobuf expires_in имеет тип int32.
	if input.ExpiresIn < -2147483648 || input.ExpiresIn > 2147483647 {
		return nil, errors.New("expires_in does not fit int32")
	}

	req := &pb.CreatePasteRequest{
		Content:    input.Content,
		Visibility: input.Visibility,
		ExpiresIn:  int32(input.ExpiresIn),
	}

	return doWithRetry(ctx, func(attemptCtx context.Context) (*CreatePasteResponse, error) {
		// Copy не позволяет менять metadata исходного контекста.
		md, _ := metadata.FromOutgoingContext(attemptCtx)
		md = md.Copy()
		md.Set("idempotency-key", idempotencyKey)
		attemptCtx = metadata.NewOutgoingContext(attemptCtx, md)

		resp, err := c.client.CreatePaste(attemptCtx, req)
		if err != nil {
			return nil, err
		}

		return &CreatePasteResponse{
			ShortCode: resp.GetShortCode(),
			URL:       resp.GetUrl(),
		}, nil
	}, isRetryableGRPC)
}

func (c *GRPCClient) GetPaste(ctx context.Context, shortCode string) (*GetPasteResponse, error) {
	req := &pb.GetPasteRequest{ShortCode: shortCode}

	return doWithRetry(ctx, func(attemptCtx context.Context) (*GetPasteResponse, error) {
		resp, err := c.client.GetPaste(attemptCtx, req)
		if err != nil {
			return nil, err
		}

		expiresAt := resp.GetExpiresAt()
		if err := expiresAt.CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid expires_at in response: %w", err)
		}

		return &GetPasteResponse{
			ShortCode:  resp.GetShortCode(),
			Content:    resp.GetContent(),
			Visibility: resp.GetVisibility(),
			ExpiresAt:  expiresAt.AsTime(),
		}, nil
	}, isRetryableGRPC)
}

func isRetryableGRPC(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
