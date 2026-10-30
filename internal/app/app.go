package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"pastebin/internal/handlers/rest"
	"pastebin/internal/repository"
	"pastebin/internal/service"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatal("Invalid database configuration")
	}

	config.MaxConns = 10
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		cancel()
		log.Fatal("Can't create database pool")
	}

	if err := pool.Ping(ctx); err != nil {
		cancel()
		pool.Close()
		log.Fatal("Can't connect to PostgreSQL")
	}

	cancel()
	defer pool.Close()

	log.Println("Connected to PostgreSQL")

	repo := repository.NewRepository(pool)
	svc := service.NewService(repo)

	restHandler := rest.NewHandler(svc)
	//grpcHandler := grpchandler.NewGrpcHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/pastes", restHandler.CreatePaste)
	mux.HandleFunc("GET /api/v1/pastes/{code}", restHandler.GetPaste)

	//httpServer := &http.Server{
	//	Addr:              ":8080",
	//	Handler:           mux,
	//	ReadHeaderTimeout: 5 * time.Second,
	//}
	//
	//// gRPC: регистрируем реализацию RPC-сервиса.
	//grpcServer := grpc.NewServer()
	//pb.RegisterPasteServiceServer(grpcServer, grpcHandler)
}
