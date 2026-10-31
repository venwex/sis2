package grpchandler

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	pb "pastebin/gen"
	e "pastebin/internal/errors"
	"pastebin/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const requestTimeout = 3 * time.Second

type GrpcHandler struct {
	pb.UnimplementedPasteServiceServer
	service service.Service
	baseURL string
}

func NewGrpcHandler(svc service.Service, baseURL string) *GrpcHandler {
	return &GrpcHandler{service: svc, baseURL: strings.TrimRight(baseURL, "/")}
}

func (h *GrpcHandler) CreatePaste(ctx context.Context, req *pb.CreatePasteRequest) (*pb.CreatePasteResponse, error) {
	values := metadata.ValueFromIncomingContext(ctx, "idempotency-key")
	if len(values) != 1 {
		return nil, status.Error(codes.InvalidArgument, "one idempotency-key is required")
	}

	input := service.CreatePasteInput{
		IdempotencyKey: values[0],
		Content:        req.GetContent(),
		Visibility:     req.GetVisibility(),
		ExpiresIn:      int(req.GetExpiresIn()),
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	paste, err := h.service.CreatePaste(ctx, &input)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.CreatePasteResponse{
		ShortCode: paste.ShortCode,
		Url:       h.baseURL + "/api/v1/pastes/" + paste.ShortCode,
	}, nil
}

func (h *GrpcHandler) GetPaste(ctx context.Context, req *pb.GetPasteRequest) (*pb.GetPasteResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	paste, err := h.service.GetPaste(ctx, req.GetShortCode())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetPasteResponse{
		ShortCode:  paste.ShortCode,
		Content:    paste.Content,
		Visibility: paste.Visibility,
		ExpiresAt:  timestamppb.New(paste.ExpiresAt),
	}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, e.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, e.ErrPasteNotFound):
		return status.Error(codes.NotFound, "paste not found")

	case errors.Is(err, e.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, "idempotency key was used with different input")

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operation timed out")

	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")

	default:
		log.Printf("gRPC request failed: %v", err)
		return status.Error(codes.Internal, "internal server error")
	}
}
