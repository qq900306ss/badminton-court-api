package main

import (
	"context"
	"log"
	"os"

	"github.com/qq900306ss/badminton-court-api/internal/handler"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

func main() {
	ctx := context.Background()
	if err := repository.Init(ctx); err != nil {
		log.Fatalf("db init: %v", err)
	}
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
