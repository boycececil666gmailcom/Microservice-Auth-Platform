package shortener

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ErrNotFound = errors.New("short URL not found")

type URLRecord struct {
	ShortURL  string    `dynamodbav:"short_url" json:"short_url"`
	LongURL   string    `dynamodbav:"long_url" json:"long_url"`
	CreatedAt time.Time `dynamodbav:"created_at" json:"created_at"`
}

type dynamoAPI interface {
	DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

type dynamoStore struct {
	client dynamoAPI
	table  string
}

func newDynamoStore(client dynamoAPI, table string) *dynamoStore {
	return &dynamoStore{client: client, table: table}
}

func (s *dynamoStore) Create(ctx context.Context, longURL string) (URLRecord, error) {
	createdAt := time.Now().UTC().Truncate(time.Millisecond)
	for _, length := range []int{16, 22, 32, 43} {
		record := URLRecord{ShortURL: shortCode(longURL, length), LongURL: longURL, CreatedAt: createdAt}
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			return URLRecord{}, err
		}
		_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(s.table),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(short_url)"),
		})
		if err == nil {
			return record, nil
		}
		var conditional *types.ConditionalCheckFailedException
		if !errors.As(err, &conditional) {
			return URLRecord{}, err
		}
		existing, getErr := s.Get(ctx, record.ShortURL)
		if getErr != nil {
			return URLRecord{}, getErr
		}
		if existing.LongURL == longURL {
			return existing, nil
		}
	}
	return URLRecord{}, errors.New("unable to allocate a collision-free short URL")
}

func (s *dynamoStore) Get(ctx context.Context, shortURL string) (URLRecord, error) {
	if !validShortURL(shortURL) {
		return URLRecord{}, fmt.Errorf("invalid short URL: %q", shortURL)
	}
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		ConsistentRead: aws.Bool(true),
		Key: map[string]types.AttributeValue{
			"short_url": &types.AttributeValueMemberS{Value: shortURL},
		},
	})
	if err != nil {
		return URLRecord{}, err
	}
	if len(output.Item) == 0 {
		return URLRecord{}, ErrNotFound
	}
	var record URLRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return URLRecord{}, err
	}
	return record, nil
}

func (s *dynamoStore) Ready(ctx context.Context) error {
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(s.table)})
	return err
}

func shortCode(longURL string, length int) string {
	digest := sha256.Sum256([]byte(longURL))
	encoded := base64.RawURLEncoding.EncodeToString(digest[:])
	if length > len(encoded) {
		length = len(encoded)
	}
	return encoded[:length]
}

func validShortURL(value string) bool {
	if len(value) < 16 || len(value) > 43 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_') {
			return false
		}
	}
	return true
}
