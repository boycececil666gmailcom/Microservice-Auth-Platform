# Read and redirect paths

```mermaid
sequenceDiagram
    participant C as Client
    participant G as API Gateway
    participant S as Shortener Lambda
    participant D as DynamoDB
    participant Q as SQS
    C->>G: GET /r/{shortURL}
    G->>S: HTTP API event
    S->>D: Strongly consistent GetItem
    D-->>S: URL record
    S->>Q: Send redirect event (1.5s maximum)
    S-->>C: 302 Location
```

Invalid identifiers return 422, unknown identifiers return 404, and dependency failures return 500. Analytics enqueue failure is logged but does not prevent a valid redirect.
