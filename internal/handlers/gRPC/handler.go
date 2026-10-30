package grpchandler

import "pastebin/internal/service"

type GrpcHandler struct {
	service service.Service
}

func NewGrpcHandler(service service.Service) *GrpcHandler {
	return &GrpcHandler{service: service}
}
