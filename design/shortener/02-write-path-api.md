# Write Path API

This document details the write path execution flow for creating shortened URLs via the serverless architecture.

### Request Flow

```mermaid
---
config:
  theme: neutral
---
sequenceDiagram
    autonumber
    participant C as Client
    participant G as Amazon API Gateway
    participant S as Shortener Lambda
    participant DB as RDS PostgreSQL
    participant R as ElastiCache Redis

    C->>G: POST /api/v1/shorten {"long_url": "https://..."}
    G->>S: Proxy Request (APIGatewayV2HTTPRequest)
    alt Invalid JSON or Invalid URL
        S-->>G: 422 Unprocessable Entity
        G-->>C: 422 Unprocessable Entity
    else Valid Request
        S->>DB: INSERT INTO urls (long_url) VALUES (...) ON CONFLICT DO NOTHING
        alt DB Execution Failure
            DB-->>S: Query Error
            S-->>G: 500 Internal Server Error
            G-->>C: 500 Internal Server Error
        else Success / Idempotent Existing
            DB-->>S: OK
            S->>DB: SELECT short_url, long_url, created_at WHERE long_url = ...
            DB-->>S: URLRecord
            S->>R: SET cached record in Redis (pre-warm)
            S-->>G: 201 Created {short_url, long_url, created_at}
            G-->>C: 201 Created JSON
        end
    end
```

### Key Technical Details
- **Idempotency**: Handled by PostgreSQL `ON CONFLICT (long_url) DO NOTHING` and `UNIQUE` constraint on `long_url`.
- **Pre-warming**: Immediately after persisting in PostgreSQL, the record is placed into Redis with a 24-hour TTL to ensure sub-millisecond latency on subsequent reads.
