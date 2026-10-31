package app

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"pastebin/internal/metrics"
	"strings"
	"syscall"
	"time"

	pb "pastebin/gen"
	grpchandler "pastebin/internal/handlers/gRPC"
	"pastebin/internal/handlers/rest"
	rediscache "pastebin/internal/redis"
	"pastebin/internal/repository"
	"pastebin/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func Run() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Println("DATABASE_URL environment variable not set")
		return
	}

	baseURL := strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	// PostgreSQL
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Println("Invalid database configuration")
		return
	}

	config.MaxConns = 10

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 5*time.Second)

	pool, err := pgxpool.NewWithConfig(connectCtx, config)
	if err != nil {
		cancelConnect()
		log.Println("Cannot create PostgreSQL pool")
		return
	}
	defer pool.Close()

	err = pool.Ping(connectCtx)
	cancelConnect()

	if err != nil {
		log.Println("Cannot connect to PostgreSQL")
		return
	}

	log.Println("Connected to PostgreSQL")

	// Redis, если недоступен при запуске — работаем без кэша.
	var cache service.Cache

	redisClient, err := rediscache.NewRedisClient(redisHost, redisPort)
	if err != nil {
		log.Printf("Redis unavailable, using PostgreSQL: %v", err)
	} else {
		defer redisClient.Close()
		cache = rediscache.NewRedisCache(redisClient)
		log.Println("Connected to Redis")
	}

	// Общие repository и service
	repo := repository.NewRepository(pool)
	svc := service.NewService(repo, cache)

	// REST
	restHandler := rest.NewHandler(svc, baseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/pastes", metrics.HTTP("create", restHandler.CreatePaste))
	mux.HandleFunc("GET /api/v1/pastes/{code}", metrics.HTTP("get", restHandler.GetPaste))
	mux.Handle("GET /metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// gRPC
	grpcHandler := grpchandler.NewGrpcHandler(svc, baseURL)

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(metrics.UnaryServerInterceptor))
	pb.RegisterPasteServiceServer(grpcServer, grpcHandler)

	grpcListener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Printf("Cannot listen on gRPC port: %v", err)
		return
	}
	defer grpcListener.Close()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 2)

	go func() {
		log.Println("Starting HTTP server on :8080")
		serverErrors <- httpServer.ListenAndServe()
	}()

	go func() {
		log.Println("Starting gRPC server on :9090")
		serverErrors <- grpcServer.Serve(grpcListener)
	}()

	select {
	case <-stopCtx.Done():
		log.Println("Shutdown requested")

	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("Server stopped: %v", err)
		}
	}

	// Даём текущим запросам до 5 секунд на завершение.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	grpcStopped := make(chan struct{})

	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
		_ = httpServer.Close()
	}

	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
		<-grpcStopped
	}

	log.Println("Servers stopped")
}
