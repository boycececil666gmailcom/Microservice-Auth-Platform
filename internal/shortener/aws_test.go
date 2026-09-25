package shortener

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func TestDynamoStoreCreatesAndReadsRecords(t *testing.T) {
	client := &fakeDynamoAPI{}
	store := newDynamoStore(client, "urls")
	record, err := store.Create(context.Background(), "https://example.com/a")
	if err != nil {
		t.Fatal(err)
	}
	if !validShortURL(record.ShortURL) || client.putInput == nil || *client.putInput.TableName != "urls" {
		t.Fatalf("unexpected create: %#v, %#v", record, client.putInput)
	}

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		t.Fatal(err)
	}
	client.getOutput = &dynamodb.GetItemOutput{Item: item}
	loaded, err := store.Get(context.Background(), record.ShortURL)
	if err != nil || loaded.LongURL != record.LongURL {
		t.Fatalf("loaded %#v: %v", loaded, err)
	}
	if _, err := store.Get(context.Background(), "bad"); err == nil {
		t.Fatal("invalid ID was accepted")
	}
}

func TestDynamoStoreReturnsExistingRecordAfterConditionalFailure(t *testing.T) {
	existing := URLRecord{
		ShortURL:  shortCode("https://example.com/a", 16),
		LongURL:   "https://example.com/a",
		CreatedAt: time.Unix(100, 0).UTC(),
	}
	item, err := attributevalue.MarshalMap(existing)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeDynamoAPI{
		putErr:    &types.ConditionalCheckFailedException{},
		getOutput: &dynamodb.GetItemOutput{Item: item},
	}
	store := newDynamoStore(client, "urls")
	record, err := store.Create(context.Background(), existing.LongURL)
	if err != nil || !record.CreatedAt.Equal(existing.CreatedAt) {
		t.Fatalf("record %#v: %v", record, err)
	}
}

func TestDynamoStorePropagatesAWSFailures(t *testing.T) {
	want := errors.New("AWS unavailable")
	client := &fakeDynamoAPI{putErr: want, describeErr: want}
	store := newDynamoStore(client, "urls")
	if _, err := store.Create(context.Background(), "https://example.com"); !errors.Is(err, want) {
		t.Fatalf("Create error = %v", err)
	}
	if err := store.Ready(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Ready error = %v", err)
	}
}

func TestSQSPublisher(t *testing.T) {
	client := &fakeSQSAPI{}
	publisher := newSQSPublisher(client, "queue-url")
	if err := publisher.PublishRedirect(context.Background(), "abcdefghijklmnop"); err != nil {
		t.Fatal(err)
	}
	if client.input == nil || *client.input.QueueUrl != "queue-url" || *client.input.MessageBody != `{"event":"redirect","short_url":"abcdefghijklmnop"}` {
		t.Fatalf("unexpected input: %#v", client.input)
	}
}

type fakeDynamoAPI struct {
	putInput    *dynamodb.PutItemInput
	putErr      error
	getOutput   *dynamodb.GetItemOutput
	getErr      error
	describeErr error
}

func (f *fakeDynamoAPI) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.putInput = input
	return &dynamodb.PutItemOutput{}, f.putErr
}

func (f *fakeDynamoAPI) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f.getOutput == nil {
		f.getOutput = &dynamodb.GetItemOutput{}
	}
	return f.getOutput, f.getErr
}

func (f *fakeDynamoAPI) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, f.describeErr
}

type fakeSQSAPI struct {
	input *sqs.SendMessageInput
	err   error
}

func (f *fakeSQSAPI) SendMessage(_ context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.input = input
	return &sqs.SendMessageOutput{}, f.err
}
