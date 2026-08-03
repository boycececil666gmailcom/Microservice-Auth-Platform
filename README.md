# Enterprise URL Shortener Platform

> High-throughput microservice-based link management and real-time click analytics platform engineered for enterprise brand marketing and instant URL redirection.

![Python](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-0.115+-009688?style=flat&logo=fastapi&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7.0+-DC382D?style=flat&logo=redis&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat&logo=postgresql&logoColor=white)
![Apache Kafka](https://img.shields.io/badge/Apache_Kafka-3.0+-231F20?style=flat&logo=apachekafka&logoColor=white)
![Terraform](https://img.shields.io/badge/Terraform-1.5+-844FBA?style=flat&logo=terraform&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

---

## 1. Core Purpose & Business Value

Enterprise URL Shortener Platform converts long, complex web links into concise, memorable brand assets while delivering real-time marketing intelligence and sub-millisecond redirection reliability.

- **Brand Recognition & Click-Through Optimization**: Transforms awkward web addresses into clean, trustworthy branded short links, dramatically improving click-through rates across marketing channels.
- **Real-Time Customer Campaign Intelligence**: Captures visitor engagement metrics, geographical reach, and channel performance immediately to optimize ad spend.
- **Enterprise High-Availability SLA**: Guarantees zero-downtime redirection performance for high-volume customer traffic spikes and seasonal promotions.
- **Security & Account Control**: Enforces precise organizational access controls, protecting enterprise links from unauthorized alterations or malicious hijacks.

---

## 2. System Architecture & Technical Execution

### Core Concept & Phased Execution Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Visitor as Visitor / User
    participant S as URL Shortener Gateway
    participant A as Click Analytics Service

    Note over Visitor, S: Phase 1: Shorten URL (Generation Path)
    Visitor->>S: Submit Long URL (POST /shorten)
    S-->>Visitor: Return Branded Short URL (e.g., https://shrt.co/xYz9b)

    Note over Visitor, S: Phase 2: Access & Redirection (Read Path)
    Visitor->>S: Click Short URL (GET /r/xYz9b)
    alt Cache Hit / Valid Mapping
        rect rgb(235, 247, 238)
            S-->>Visitor: HTTP 302 Redirect (Location: https://example.com/target)
        end
    else Cache Miss / DB Fallback
        rect rgb(255, 243, 205)
            S->>S: Fetch from Database & Warm Cache
            S-->>Visitor: HTTP 302 Redirect
        end
    else Link Not Found
        rect rgb(253, 237, 237)
            S-->>Visitor: HTTP 404 Not Found
        end
    end
    
    Note over S, A: Phase 3: Asynchronous Tracking (Background Path)
    S-)+A: Capture click event via Queue (stats count +1)
    deactivate A
```

### High-Level Target Production Architecture Diagram

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
        LB["Load Balancer<br/>(Nginx / HAProxy)"]
    end

    subgraph WritePath["Write Path"]
        APIGW["API Gateway<br/>(FastAPI)"]
    end

    subgraph ReadPath["Read Path"]
        Redirect["API Gateway<br/>(FastAPI)"]
    end

    subgraph AuthSvc["Auth Service"]
        Auth["Auth Handler<br/>(RS256 Private Key)"]
        subgraph AuthDB["Owned Storage"]
            UserDB[("User DB<br/>(PostgreSQL)")]
        end
    end

    subgraph ShortenerSvc["Shortener Service"]
        Shortener["Shortener Handler<br/>(FastAPI + Uvicorn)"]
        subgraph ShortenerDB["Owned Storage"]
            Redis["Cache<br/>(Redis)"]
            Primary[("Primary DB<br/>(PostgreSQL)")]
            Replica[("Replica DB<br/>(PostgreSQL)")]
        end
    end

    subgraph Async
        Queue["Queue<br/>(Kafka / RabbitMQ / SQS)"]
        Analytics["Analytics Service<br/>(ClickHouse / Elasticsearch)"]
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

### Container Network & Isolation Design Diagram

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

---

## 3. Repository Structure

```text
URL-Shortener/
├── .agents/          # Workspace configuration and guidelines
├── design/           # Architecture diagrams and design specifications
│   ├── analytics/    # Analytics service design documentation
│   ├── auth/         # Authentication service design documentation
│   └── shortener/    # Shortener service design documentation
├── infra_tf/         # Infrastructure as Code (Terraform)
│   ├── apps.tf       # Application workloads deployment
│   ├── dbs.tf        # Databases & cache cluster setup
│   ├── main.tf       # Terraform provider configuration
│   ├── outputs.tf    # Infrastructure output definitions
│   └── variables.tf  # Environment variable declarations
├── keys/             # RSA public/private keys for JWT verification
├── scripts/          # Automation build and deployment scripts
│   ├── 01_build_images.sh
│   ├── 02_deploy_tf.sh
│   ├── 03_run_tests.sh
│   └── run_test_k8s.sh
├── services/         # Decoupled microservices architecture
│   ├── analytics/    # Real-time click tracking & aggregation
│   ├── auth/         # JWT authentication & user management
│   ├── gateway/      # Reverse proxy & dynamic request router
│   └── shortener/    # URL encoding, decoding & cache layer
├── tests/            # Automated test suites
│   ├── e2e/          # End-to-end user journey tests
│   ├── integration/  # Inter-service integration tests
│   └── unit/         # Unit tests per microservice
├── .dockerignore
├── .gitignore
├── pyproject.toml    # Dependency management & pytest config
└── uv.lock           # Locked dependency lockfile
```
