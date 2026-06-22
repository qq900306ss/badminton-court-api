package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/qq900306ss/badminton-court-api/internal/handler"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// V2 adapter: Lambda Function URLs use payload format 2.0 (same as API
// Gateway HTTP API v2). The v1 adapter reads event.Path, which is empty in
// v2 events, so every request collapses to "/" — must use NewV2 here.
var ginLambda *ginadapter.GinLambdaV2

func init() {
	ctx := context.Background()
	// Non-fatal: log DB init problems but still start the function so /health
	// works and DB errors surface as 500s (with detail) instead of a 502 crash.
	if err := repository.Init(ctx); err != nil {
		log.Printf("db init: %v", err)
	}
	ginLambda = ginadapter.NewV2(handler.NewRouter())
}

func main() {
	lambda.Start(ginLambda.ProxyWithContext)
}
