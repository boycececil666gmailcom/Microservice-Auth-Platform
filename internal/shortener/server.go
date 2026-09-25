package shortener

import (
	"context"
	"errors"
	"net/http"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Config struct {
	URLsTable   string
	SQSQueueURL string
}

type urlStore interface {
	Create(context.Context, string) (URLRecord, error)
	Get(context.Context, string) (URLRecord, error)
	Ready(context.Context) error
}

type eventPublisher interface {
	PublishRedirect(context.Context, string) error
}

type Server struct {
	store     urlStore
	publisher eventPublisher
}

// ConfigFromEnv validates the service's environment configuration.
func ConfigFromEnv() (Config, error) {
	table := os.Getenv("URLS_TABLE")
	if table == "" {
		return Config{}, errors.New("URLS_TABLE is required")
	}
	return Config{URLsTable: table, SQSQueueURL: os.Getenv("SQS_QUEUE_URL")}, nil
}

// NewServer creates the AWS-backed URL shortener.
func NewServer(ctx context.Context, config Config) (*Server, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	store := newDynamoStore(dynamodb.NewFromConfig(awsConfig), config.URLsTable)
	var publisher eventPublisher = discardPublisher{}
	if config.SQSQueueURL != "" {
		publisher = newSQSPublisher(sqs.NewFromConfig(awsConfig), config.SQSQueueURL)
	}
	return newServer(store, publisher), nil
}

func newServer(store urlStore, publisher eventPublisher) *Server {
	if publisher == nil {
		publisher = discardPublisher{}
	}
	return &Server{store: store, publisher: publisher}
}

// Handler returns all shortener routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /r/{shortURL}", s.redirect)
	mux.HandleFunc("POST /api/v1/shorten", s.shorten)
	mux.HandleFunc("GET /api/v1/urls/{shortURL}", s.lookup)
	return mux
}
