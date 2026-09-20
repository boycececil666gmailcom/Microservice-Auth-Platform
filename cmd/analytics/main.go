// #region Analytics Main
package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/analytics"
)

// main runs the analytics AWS Lambda handler handling SQS events and HTTP API proxying.
func main() {
	service := analytics.NewServer()
	defer service.Close()

	adapter := httpadapter.NewV2(service.Handler())
	lambda.Start(func(ctx context.Context, raw json.RawMessage) (any, error) {
		var sqsEvent events.SQSEvent
		if err := json.Unmarshal(raw, &sqsEvent); err == nil && len(sqsEvent.Records) > 0 && sqsEvent.Records[0].EventSource == "aws:sqs" {
			for _, r := range sqsEvent.Records {
				var event struct {
					ShortURL int64  `json:"short_url"`
					Event    string `json:"event"`
				}
				if err := json.Unmarshal([]byte(r.Body), &event); err == nil && event.ShortURL > 0 {
					service.RecordRedirect(event.ShortURL)
				}
			}
			return map[string]string{"status": "ok"}, nil
		}

		var req events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(raw, &req); err == nil && req.RawPath != "" {
			return adapter.ProxyWithContext(ctx, req)
		}

		return nil, errors.New("unrecognized lambda event payload")
	})
}

// #endregion

