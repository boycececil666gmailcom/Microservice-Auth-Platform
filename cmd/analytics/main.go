package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/boycececil666gmailcom/url-shortener/internal/analytics"
)

func main() {
	config, err := analytics.ConfigFromEnv()
	if err != nil {
		slog.Error("invalid analytics configuration", "error", err)
		os.Exit(1)
	}
	service, err := analytics.NewServer(context.Background(), config)
	if err != nil {
		slog.Error("analytics startup failed", "error", err)
		os.Exit(1)
	}
	adapter := httpadapter.NewV2(service.Handler())
	lambda.Start(func(ctx context.Context, raw json.RawMessage) (any, error) {
		var sqsEvent events.SQSEvent
		if err := json.Unmarshal(raw, &sqsEvent); err == nil && len(sqsEvent.Records) > 0 && sqsEvent.Records[0].EventSource == "aws:sqs" {
			return service.ProcessSQSEvent(ctx, sqsEvent), nil
		}
		var request events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(raw, &request); err == nil && request.RawPath != "" {
			return adapter.ProxyWithContext(ctx, request)
		}
		return nil, errors.New("unrecognized lambda event payload")
	})
}
