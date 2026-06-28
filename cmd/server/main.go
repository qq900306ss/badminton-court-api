package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/qq900306ss/badminton-court-api/internal/handler"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
	"github.com/qq900306ss/badminton-court-api/internal/service"
)

func main() {
	ctx := context.Background()
	if err := repository.Init(ctx); err != nil {
		log.Fatalf("db init: %v", err)
	}
	// real scheduler: close sessions 2h past end_at even when nobody's looking
	// (always-on server makes this trivial — no AWS EventBridge needed)
	service.StartAutoCloseSweeper(ctx, 10*time.Minute)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r := handler.NewRouter()
	log.Printf("server running on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
