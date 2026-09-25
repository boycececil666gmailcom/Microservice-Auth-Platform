package shortener

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type sqsAPI interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type sqsPublisher struct {
	client   sqsAPI
	queueURL string
}

func newSQSPublisher(client sqsAPI, queueURL string) sqsPublisher {
	return sqsPublisher{client: client, queueURL: queueURL}
}

func (p sqsPublisher) PublishRedirect(ctx context.Context, shortURL string) error {
	payload, err := json.Marshal(map[string]string{"short_url": shortURL, "event": "redirect"})
	if err != nil {
		return err
	}
	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(payload)),
	})
	return err
}

type discardPublisher struct{}

func (discardPublisher) PublishRedirect(context.Context, string) error { return nil }
