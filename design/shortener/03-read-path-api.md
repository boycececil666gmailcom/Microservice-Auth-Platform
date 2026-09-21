# Read Path API & Redirect Flow

This document details the read resolution and redirect workflow through Amazon API Gateway, AWS Lambda, ElastiCache Redis, and RDS PostgreSQL.

## 1. Metadata Query Flow (`GET /api/v1/urls/{shortURL}`)

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant G as Amazon API Gateway
    participant S as Shortener Lambda
    participant R as ElastiCache Redis
    participant DB as RDS PostgreSQL

    C->>G: GET /api/v1/urls/{shortURL}
    G->>S: Proxy Request (APIGatewayV2HTTPRequest)
    S->>R: GET url:{shortURL}
    alt Cache Hit
        R-->>S: Return cached URLRecord JSON
    else Cache Miss or Redis Error
        S->>DB: SELECT short_url, long_url, created_at WHERE short_url = ...
        alt Not Found in DB
            DB-->>S: pgx.ErrNoRows
            S-->>G: 404 Not Found
            G-->>C: 404 Not Found
        else Found
            DB-->>S: URLRecord
            S->>R: SET url:{shortURL} with 24h TTL (warm cache)
        end
    end
    S-->>G: 200 OK {short_url, long_url, created_at}
    G-->>C: 200 OK JSON
```

---

## 2. Public Redirect Flow (`GET /r/{shortURL}`)

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Browser)
    participant G as Amazon API Gateway
    participant S as Shortener Lambda
    participant Q as Amazon SQS Queue

    C->>G: GET /r/{shortURL}
    G->>S: Proxy Request (APIGatewayV2HTTPRequest)
    S->>S: Resolve URL (Redis cache-aside -> PostgreSQL fallback)
    alt Resolved Successfully
        S->>Q: AWS SDK v2 SendMessage {short_url, event: "redirect"}
        S-->>G: HTTP 302 Found (Location: long_url)
        G-->>C: HTTP 302 Redirect to destination
    else URL Invalid or Not Found
        S-->>G: HTTP 404 or 422
        G-->>C: Error response
    end
```
