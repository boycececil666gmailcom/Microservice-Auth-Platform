# Analytics Service Design

This document details the design and event-driven architecture of the **Analytics Service** using **Apache Kafka** as the asynchronous message broker.

## 1. Redirect Tracking & Event Processing Flow

When a user accesses a shortened link, the read path processes the redirect immediately. It then publishes an event to Kafka asynchronously to ensure zero impact on the redirect latency.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Browser)
    participant G as API Gateway
    participant S as Shortener Service
    participant K as Kafka Broker
    participant A as Analytics Service

    C->>G: GET /r/{short_url}
    G->>S: Forward request to Shortener
    S->>S: Fetch URL (Redis Cache or Postgres)
    S-->>G: HTTP 302 (Redirect Target)
    G-->>C: HTTP 302 Redirect Target

    Note over S,K: Async Event Dispatch (Background Task)
    S->>K: Publish Event "url-redirects" {short_url, event: "redirect"}

    Note over K,A: Event Ingestion (Background Consumer)
    K->>A: Push Event to Consumer
    A->>A: Update In-Memory Counts (total_redirects, redirects_by_short_url)
```

---

## 2. Analytics Retrieval Flow

The client retrieves analytics data via the API Gateway. The gateway validates the RS256 JWT signature using the Auth service's cached JWKS and proxies the stats request to the analytics container.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Analytics Service

    C->>G: GET /api/v1/analytics/stats
    Note right of C: Header: Authorization: Bearer <Consolidated_JWT>
    
    G->>G: Verify RS256 JWT signature using cached JWKS
    
    G->>A: Forward stats query to http://analytics:8003/stats
    A-->>G: Returns analytics counts JSON
    G-->>C: 200 OK with analytics data
```

---

## 3. Data Schema

### Kafka Event Structure
The message published on the `url-redirects` topic is a serialized JSON payload containing the numeric short URL ID:

```json
{
  "short_url": 42,
  "event": "redirect"
}
```

### Stats API Payload Structure
The HTTP endpoint `/stats` returns a summary of the captured statistics:

```json
{
  "total_redirects": 1,
  "redirects_by_short_url": {
    "42": 1
  }
}
```
