# Microservice Authentication Platform

> Enterprise-grade identity and access control platform built on a decoupled microservice architecture — enabling organizations to secure product features, unify user identities across sign-in providers, and maintain complete session lifecycle control.

![Python](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-0.115+-009688?style=flat&logo=fastapi&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-RS256-000000?style=flat&logo=jsonwebtokens&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7.0+-DC382D?style=flat&logo=redis&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat&logo=postgresql&logoColor=white)
![Apache Kafka](https://img.shields.io/badge/Apache_Kafka-3.0+-231F20?style=flat&logo=apachekafka&logoColor=white)
![Terraform](https://img.shields.io/badge/Terraform-1.5+-844FBA?style=flat&logo=terraform&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

---

## 1. Core Purpose & Business Value

This platform delivers a complete user identity and session management foundation, enabling product teams to ship authenticated features faster while giving security teams full control over who accesses what — and for how long.

- **Frictionless Onboarding & Social Sign-in**: Users can register or sign in using their existing Google accounts with a single click, dramatically reducing sign-up abandonment and increasing conversion rates for consumer-facing products.
- **Enterprise Account Control & Immediate Access Revocation**: Security teams can instantly invalidate any user session organization-wide — critical for offboarding employees, responding to suspicious activity, or enforcing compliance policies — without requiring users to wait for natural token expiration.
- **Zero-Trust Feature Protection**: Every product feature behind authentication is protected by a cryptographically verifiable identity check that requires no additional server calls per request, enabling sub-millisecond access decisions at scale without sacrificing security.
- **Unified Identity Across Sign-in Providers**: Whether a user registers via email/password or Google, the platform issues a single, consistent identity token accepted by all internal services — eliminating fragmented user records and duplicate account issues.
- **Stateless Horizontal Scalability**: The access verification layer operates entirely without shared state between server instances, allowing the product to scale to any number of concurrent users without bottlenecks in the authentication path.

---

## 2. System Architecture & Technical Execution

The platform separates the identity issuance concern (Auth Service) from the verification concern (API Gateway), enabling stateless horizontal scaling of the request path while centralizing all credential management in a single, auditable service.

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
        APIGW["API Gateway<br/>(FastAPI + Uvicorn)<br/>RS256 Public Key Verification"]
    end

    subgraph ReadPath["Read Path"]
        Redirect["API Gateway<br/>(FastAPI + Uvicorn)<br/>RS256 Public Key Verification"]
    end

    subgraph AuthSvc["Auth Service"]
        Auth["Auth Handler<br/>(RS256 Private Key Signing)<br/>Google OIDC + Local Auth"]
        subgraph AuthDB["Owned Storage"]
            UserDB[("User DB<br/>(PostgreSQL)")]
            AuthRedis["Session Store<br/>(Redis)<br/>refresh_token:{token} TTL 30d"]
        end
    end

    subgraph ShortenerSvc["Shortener Service"]
        Shortener["Shortener Handler<br/>(FastAPI + Uvicorn)"]
        subgraph ShortenerDB["Owned Storage"]
            Redis["Cache<br/>(Redis)<br/>url:{id} TTL 24h"]
            Primary[("Primary DB<br/>(PostgreSQL)")]
            Replica[("Replica DB<br/>(PostgreSQL)")]
        end
    end

    subgraph Async
        Queue["Queue<br/>(Apache Kafka)"]
        Analytics["Analytics Service<br/>(FastAPI + Uvicorn)"]
    end

    User --> APIGW
    APIGW --> Auth
    APIGW --> Shortener

    Auth --> UserDB
    Auth --> AuthRedis

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
            GoogleCbH["POST /auth/google/callback"]
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
            TokenStore["key: refresh_token:{token}<br/>value: user_email<br/>TTL: 30d"]
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
    GoogleCbH -->|"SELECT / INSERT"| UserPG
    GoogleCbH -->|"SET refresh_token:{token}"| TokenStore
```

---

## 3. Repository Structure

```text
Microservice-Auth-Platform/
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
├── scripts/          # Automation build and deployment scripts
│   ├── 01_build_images.sh
│   ├── 02_deploy_tf.sh
│   ├── 03_run_tests.sh
│   └── run_test_k8s.sh
├── services/         # Decoupled microservices architecture
│   ├── analytics/    # Real-time click tracking & aggregation (Kafka consumer)
│   ├── auth/         # Identity issuance — JWT RS256, Google OIDC, session lifecycle
│   │   ├── config.py     # ENV-driven config: RSA keys, Redis URL, OIDC credentials
│   │   ├── database.py   # asyncpg connection pool & auto-migration
│   │   ├── main.py       # FastAPI app — login, refresh, logout, Google OIDC
│   │   ├── oidc.py       # Google OAuth2 URL builder, ID token parser & mock fixtures
│   │   ├── passwords.py  # bcrypt hash & verify
│   │   ├── schemas.py    # Pydantic request/response models
│   │   └── tokens.py     # RS256 JWT creation, opaque refresh token, Redis store
│   ├── gateway/      # Reverse proxy — RS256 public key verification (zero network call)
│   └── shortener/    # URL encoding, decoding & Redis cache-aside layer
├── tests/            # Automated test suites
│   ├── e2e/          # End-to-end user journey tests (full Docker stack)
│   ├── integration/  # Inter-service integration tests (Redis token store)
│   └── unit/         # Unit tests — bcrypt, RS256 JWT, Google OIDC token parsing
├── .dockerignore
├── .gitignore
├── pyproject.toml    # Dependency management & pytest config
└── uv.lock           # Locked dependency lockfile
```
