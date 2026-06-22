package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/qq900306ss/badminton-court-api/internal/handler"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

var ginLambda *ginadapter.GinLambda

func init() {
	ctx := context.Background()
	if err := repository.Init(ctx); err != nil {
		log.Fatalf("db init: %v", err)
	}
	ginLambda = ginadapter.New(handler.NewRouter())
}

func main() {
	lambda.Start(ginLambda.ProxyWithContext)
}
