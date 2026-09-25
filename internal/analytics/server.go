package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Config struct {
	TableName string
	DedupeTTL time.Duration
}

type Stats struct {
	TotalRedirects      int64            `json:"total_redirects"`
	RedirectsByShortURL map[string]int64 `json:"redirects_by_short_url"`
}

type counterStore interface {
	Increment(context.Context, string, string, time.Time) error
	Snapshot(context.Context) (Stats, error)
	Ready(context.Context) error
}

type Server struct {
	store     counterStore
	dedupeTTL time.Duration
}

func ConfigFromEnv() (Config, error) {
	table := os.Getenv("ANALYTICS_TABLE")
	if table == "" {
		return Config{}, errors.New("ANALYTICS_TABLE is required")
	}
	ttl := 15 * 24 * time.Hour
	if raw := os.Getenv("DEDUPE_TTL_SECONDS"); raw != "" {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || seconds < 14*24*60*60 || seconds > 30*24*60*60 {
			return Config{}, errors.New("DEDUPE_TTL_SECONDS must be between 14 and 30 days")
		}
		ttl = time.Duration(seconds) * time.Second
	}
	return Config{TableName: table, DedupeTTL: ttl}, nil
}

func NewServer(ctx context.Context, config Config) (*Server, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return newServer(newDynamoCounterStore(dynamodb.NewFromConfig(awsConfig), config.TableName), config.DedupeTTL), nil
}

func newServer(store counterStore, dedupeTTL time.Duration) *Server {
	return &Server{store: store, dedupeTTL: dedupeTTL}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /api/v1/analytics/stats", s.getStats)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ready(ctx); err != nil {
		slog.Warn("DynamoDB readiness check failed", "error", err)
		errorJSON(w, http.StatusServiceUnavailable, "Data store unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) RecordRedirect(ctx context.Context, shortURL, eventID string) error {
	if !validShortURL(shortURL) || eventID == "" {
		return errors.New("invalid redirect event")
	}
	return s.store.Increment(ctx, shortURL, eventID, time.Now().Add(s.dedupeTTL))
}

// ProcessSQSEvent records valid messages and reports only failed records for retry.
func (s *Server) ProcessSQSEvent(ctx context.Context, event events.SQSEvent) events.SQSEventResponse {
	response := events.SQSEventResponse{}
	for _, record := range event.Records {
		var payload struct {
			ShortURL string `json:"short_url"`
			Event    string `json:"event"`
		}
		err := json.Unmarshal([]byte(record.Body), &payload)
		if err == nil && payload.Event == "redirect" {
			err = s.RecordRedirect(ctx, payload.ShortURL, record.MessageId)
		}
		if err != nil || payload.Event != "redirect" {
			slog.Warn("Redirect event processing failed", "message_id", record.MessageId, "error", err)
			response.BatchItemFailures = append(response.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
		}
	}
	return response
}

func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	stats, err := s.store.Snapshot(ctx)
	if err != nil {
		slog.Error("Analytics snapshot failed", "error", err)
		errorJSON(w, http.StatusServiceUnavailable, "Analytics unavailable")
		return
	}
	writeJSON(w, http.StatusOK, stats)
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errorJSON(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

type dynamoCounterAPI interface {
	DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

type dynamoCounterStore struct {
	client dynamoCounterAPI
	table  string
}

func newDynamoCounterStore(client dynamoCounterAPI, table string) *dynamoCounterStore {
	return &dynamoCounterStore{client: client, table: table}
}

func (s *dynamoCounterStore) Increment(ctx context.Context, shortURL, eventID string, expiresAt time.Time) error {
	_, err := s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName: aws.String(s.table),
				Item: map[string]types.AttributeValue{
					"metric_key": &types.AttributeValueMemberS{Value: "EVENT#" + eventID},
					"expires_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt.Unix(), 10)},
				},
				ConditionExpression: aws.String("attribute_not_exists(metric_key)"),
			}},
			counterUpdate(s.table, "TOTAL"),
			counterUpdate(s.table, "URL#"+shortURL),
		},
	})
	if err == nil {
		return nil
	}
	var canceled *types.TransactionCanceledException
	if !errors.As(err, &canceled) {
		return err
	}
	output, getErr := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		ConsistentRead: aws.Bool(true),
		Key: map[string]types.AttributeValue{
			"metric_key": &types.AttributeValueMemberS{Value: "EVENT#" + eventID},
		},
	})
	if getErr == nil && len(output.Item) != 0 {
		return nil
	}
	return err
}

func counterUpdate(table, key string) types.TransactWriteItem {
	return types.TransactWriteItem{Update: &types.Update{
		TableName:                aws.String(table),
		Key:                      map[string]types.AttributeValue{"metric_key": &types.AttributeValueMemberS{Value: key}},
		UpdateExpression:         aws.String("SET #count = if_not_exists(#count, :zero) + :one"),
		ExpressionAttributeNames: map[string]string{"#count": "count"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
	}}
}

func (s *dynamoCounterStore) Snapshot(ctx context.Context) (Stats, error) {
	stats := Stats{RedirectsByShortURL: make(map[string]int64)}
	input := &dynamodb.ScanInput{
		TableName:        aws.String(s.table),
		FilterExpression: aws.String("metric_key = :total OR begins_with(metric_key, :url)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":total": &types.AttributeValueMemberS{Value: "TOTAL"},
			":url":   &types.AttributeValueMemberS{Value: "URL#"},
		},
		ProjectionExpression:     aws.String("metric_key, #count"),
		ExpressionAttributeNames: map[string]string{"#count": "count"},
	}
	for {
		output, err := s.client.Scan(ctx, input)
		if err != nil {
			return Stats{}, err
		}
		for _, item := range output.Items {
			keyValue, keyOK := item["metric_key"].(*types.AttributeValueMemberS)
			countValue, countOK := item["count"].(*types.AttributeValueMemberN)
			if !keyOK || !countOK {
				continue
			}
			count, err := strconv.ParseInt(countValue.Value, 10, 64)
			if err != nil {
				continue
			}
			if keyValue.Value == "TOTAL" {
				stats.TotalRedirects = count
			} else if strings.HasPrefix(keyValue.Value, "URL#") {
				stats.RedirectsByShortURL[strings.TrimPrefix(keyValue.Value, "URL#")] = count
			}
		}
		if len(output.LastEvaluatedKey) == 0 {
			break
		}
		input.ExclusiveStartKey = output.LastEvaluatedKey
	}
	return stats, nil
}

func (s *dynamoCounterStore) Ready(ctx context.Context) error {
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(s.table)})
	return err
}
