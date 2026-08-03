# Core Concept

A URL Shortener is a service that translates long, complex URLs into short, clean aliases (slugs) to make them easier to share and track.

### How it works:
*   **Generation (Write Path)**: A user submits a long URL. The system generates a unique slug (e.g., Base62 encoding of an auto-incrementing ID or hash), saves the mapping in the database, and returns the shortened URL.
*   **Redirection (Read Path)**: When a user visits the short URL, the gateway performs a fast lookup (caching the mapping in memory/Redis to minimize database load) and returns an HTTP `302 Found` (or `301 Moved Permanently`) redirect to the original URL.
*   **Analytics (Async Path)**: While redirecting, the system triggers a background tracking event (e.g., via a message queue like Kafka) to log click information and compile reports without delaying the redirect.

> A simple flow showing how the URL shortener operates: creating a short link, redirecting visitors, and collecting click stats.

```mermaid
sequenceDiagram
    autonumber
    actor Visitor as Visitor / User
    participant S as URL Shortener
    participant A as Click Analytics

    Note over Visitor, S: Phase 1: Shorten URL
    Visitor->>S: Submit Long URL (e.g., https://example.com/very-long-path?id=123)
    S-->>Visitor: Return Branded Short URL (e.g., https://shrt.co/xYz9b)

    Note over Visitor, S: Phase 2: Access & Tracking
    Visitor->>S: Click Short URL (https://shrt.co/xYz9b)
    S-->>Visitor: HTTP 302 Redirect (Location: https://example.com/very-long-path?id=123)
    
    Note over S, A: Asynchronous Tracking (Background)
    S-)+A: Capture click event (stats count +1)
    deactivate A
```

---

# High-Level Architecture

> This diagram shows the **target production design**

```mermaid
---
config:
  layout: elk
  theme: neutral
---
flowchart TB

    subgraph Client
        User["Browser / Mobile App<br/>(React / Next.js)"]
    end

    subgraph Edge
        CDN["CDN<br/>(Cloudflare / AWS CloudFront)"]
        LB["Load balancer<br/>(Nginx / HAProxy)"]
    end

    subgraph WritePath["Write Path"]
        APIGW["API gateway<br/>(FastAPI)"]
    end

    subgraph ReadPath["Read Path"]
        Redirect["API gateway<br/>(FastAPI)"]
    end

    subgraph AuthSvc["Auth Service"]
        Auth["Auth handler<br/>(RS256 Private Key)"]
        subgraph AuthDB["Owned Storage"]
            UserDB[("User DB<br/>(PostgreSQL)")]
        end
    end

    subgraph ShortenerSvc["Shortener Service"]
        Shortener["Shortener handler<br/>(FastAPI + Uvicorn )"]
        subgraph ShortenerDB["Owned Storage"]
            Redis["Cache<br/>(Redis)"]
            Primary[("Primary DB<br/>(PostgreSQL)")]
            Replica[("Replica DB<br/>(PostgreSQL)")]
        end
    end

    subgraph Async
        Queue["Queue<br/>(Kafka / RabbitMQ / SQS)"]
        Analytics["Analytics service<br/>(ClickHouse / Elasticsearch)"]
    end

    User --> APIGW
    APIGW --> Auth
    APIGW --> Shortener

    Auth --> UserDB

    Shortener --> Redis
    Shortener --> Primary
    Primary --> Replica
    Redis -. Cache Miss .-> Replica

    User --> CDN
    CDN --> LB
    LB --> Redirect

    Redirect --> Shortener

    Redirect --> Queue
    Queue --> Analytics
```

---

# Container Design

> Shows the **running containers**, which communicate only over the internal Docker bridge network, and what is exposed to the outside world.

```mermaid
---
config:
  layout: elk
  theme: neutral
---
flowchart TB

    subgraph Outside["Outside World"]
        ExternalClient["curl / Browser / pytest"]
    end

    subgraph Exposed["Exposed to Host"]
        GW["gateway<br/>(FastAPI + Uvicorn)<br/>RS256 Public Key JWT Verification<br/>port 8000"]
    end

    subgraph Internal["Docker Internal Network - not reachable from host"]

        subgraph ShortenerCtr["shortener (FastAPI + Uvicorn, port 8001)"]
            direction TB
            WriteH["POST /shorten"]
            ReadH["GET /urls/:id<br/>GET /r/:id"]
        end

        subgraph AuthCtr["auth (FastAPI + Uvicorn, port 8002)"]
            direction TB
            LoginH["POST /auth/login"]
            RefreshH["POST /auth/refresh"]
            LogoutH["POST /auth/logout"]
        end

        subgraph AnalyticsCtr["analytics (FastAPI + Uvicorn, port 8003)"]
            direction TB
            StatsH["GET /stats"]
            ConsumeH["Kafka Consumer"]
        end

        subgraph KafkaCtr["kafka (Apache Kafka, port 9092)"]
            Topic["topic: url-redirects"]
        end

        subgraph RedisCtr["shortener-redis (Redis 7, port 6379)"]
            Cache["key: url:{id}<br/>TTL: 24h"]
        end

        subgraph DBCtr["db (PostgreSQL 16, port 5432)"]
            PG[("table: urls")]
        end

        subgraph AuthRedisCtr["auth-redis (Redis 7, port 6379)"]
            TokenStore["key: refresh_token:{token}<br/>value: user_id<br/>TTL: 30d"]
        end

        subgraph AuthDBCtr["auth-db (PostgreSQL 16, port 5432)"]
            UserPG[("table: users")]
        end

    end

    ExternalClient -->|"port 8000 - only exposed port"| GW
    GW -->|"httpx - internal network only"| ShortenerCtr
    GW -->|"httpx - internal network only"| AuthCtr
    GW -->|"httpx - internal network only"| AnalyticsCtr
    WriteH -->|"INSERT ON CONFLICT"| PG
    ReadH -->|"GET url:{id}"| Cache
    Cache -.->|"Cache MISS"| PG
    PG -.->|"Cache WARM"| Cache
    ReadH -->|"Publish event (async)"| KafkaCtr
    KafkaCtr -.->|"Consume event"| ConsumeH
    LoginH -->|"SELECT / INSERT"| UserPG
    LoginH -->|"SET refresh_token:{token}"| TokenStore
    RefreshH -->|"GET refresh_token:{token}"| TokenStore
    LogoutH -->|"DEL refresh_token:{token}"| TokenStore
```
