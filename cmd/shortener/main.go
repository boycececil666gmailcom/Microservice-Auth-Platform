// #region Shortener Main
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/shortener"
)

// main validates configuration, starts the shortener service, and runs the AWS Lambda handler.
func main() {
	config, err := shortener.ConfigFromEnv()
	if err != nil {
		slog.Error("[Shortener-main] invalid shortener configuration", "error", err)
		os.Exit(1)
	}
	service, err := shortener.NewServer(context.Background(), config)
	if err != nil {
		slog.Error("[Shortener-main] shortener startup failed", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	adapter := httpadapter.NewV2(service.Handler())
	lambda.Start(adapter.ProxyWithContext)
}

// #endregion

