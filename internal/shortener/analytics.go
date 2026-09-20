package shortener

// #region Analytics Producer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// enqueueAnalytics publishes a redirect event directly to AWS SQS.
func (s *Server) enqueueAnalytics(shortURL int64) {
	if s.sqs == nil || s.config.SQSQueueURL == "" {
		return
	}
	payload, err := json.Marshal(map[string]any{"short_url": shortURL, "event": "redirect"})
	if err != nil {
		slog.Error("[Shortener-enqueueAnalytics] event serialization failed", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = s.sqs.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.config.SQSQueueURL),
		MessageBody: aws.String(string(payload)),
	})
	if err != nil {
		slog.Warn("[Shortener-enqueueAnalytics] SQS send message failed", "error", err)
	}
}

// #endregion
