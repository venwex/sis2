package main

import (
	"context"
	"log"

	"pastebin/internal/client"

	"github.com/google/uuid"
)

func main() {
	restClient := client.NewRESTClient("http://localhost:8080")

	grpcClient, err := client.NewGRPCClient("localhost:9090")
	if err != nil {
		log.Println("Create gRPC client:", err)
		return
	}
	defer grpcClient.Close()

	ctx := context.Background()

	input := client.CreatePasteRequest{
		Content:    "Hello from REST and gRPC",
		Visibility: "public",
		ExpiresIn:  3600,
	}

	// Одна логическая операция — один ключ для обоих интерфейсов
	key := uuid.NewString()

	// Создание через REST
	restCreated, err := restClient.CreatePaste(ctx, input, key)
	if err != nil {
		log.Println("REST create:", err)
		return
	}

	log.Printf("REST created: %s", restCreated.URL)

	// Повтор той же операции через gRPC
	grpcCreated, err := grpcClient.CreatePaste(ctx, input, key)
	if err != nil {
		log.Println("gRPC create:", err)
		return
	}

	log.Printf("gRPC returned: %s", grpcCreated.URL)

	if restCreated.ShortCode != grpcCreated.ShortCode {
		log.Println("ERROR: REST and gRPC returned different short codes")
		return
	}

	log.Println("Idempotency: both interfaces returned the same short code")

	// Чтение через REST
	restPaste, err := restClient.GetPaste(ctx, restCreated.ShortCode)
	if err != nil {
		log.Println("REST get:", err)
		return
	}

	log.Printf("REST content: %s", restPaste.Content)

	// Чтение через gRPC
	grpcPaste, err := grpcClient.GetPaste(ctx, restCreated.ShortCode)
	if err != nil {
		log.Println("gRPC get:", err)
		return
	}

	log.Printf("gRPC content: %s", grpcPaste.Content)

	if restPaste.Content != input.Content || grpcPaste.Content != input.Content {
		log.Println("ERROR: returned content differs from the original")
		return
	}

	log.Println("Both interfaces returned the original content")
}
