# Microservice Authentication Platform

> Production-grade authentication microservice implementing JWT RS256 asymmetric signing, Google OpenID Connect (OIDC), Redis-backed refresh token rotation, and bcrypt password hashing — deployed as an isolated Docker container behind an API Gateway with zero-trust local token verification.

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

## 1. Authentication Architecture & Design Decisions

This platform implements a **dual authentication strategy** — local password auth and Google OIDC — unified under a single RS256 JWT schema. The key design decisions are documented below.

### Why RS256 (Asymmetric) Instead of HS256 (Symmetric)?

| Aspect | RS256 (this project) | HS256 |
|--------|----------------------|-------|
| Key type | RSA private + public key pair | Single shared secret |
| Token issuer | Signs with private key (Auth Service only) | Any party with the secret can sign |
| Token verifier | Verifies with public key (Gateway, any service) | Must share the same secret |
| Security boundary | Private key never leaves Auth container | Secret must be distributed to all verifiers |
| **Use case** | **Microservices, federated identity** | Single-service monolith |

> **In this project**: The API Gateway verifies every incoming JWT **locally** using the RS256 public key — zero network round-trips to the Auth Service per request. This is the same model used by Google, Auth0, and AWS Cognito.

### Auth Flow: Login / Sign-up (Dual-Provider)

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant GW as API Gateway<br/>(RS256 Public Key)
    participant Auth as Auth Service<br/>(RS256 Private Key)
    participant PG as PostgreSQL<br/>(users table)
    participant RD as Redis<br/>(refresh_token:{token} → email)

    Note over C, GW: Path A — Password Authentication (POST /auth/login)
    C->>GW: POST /auth/login { email, password }
    GW->>Auth: Forward request (no auth required)
    Auth->>PG: SELECT password_hash WHERE email = $1
    alt User not found (Sign-up)
        Auth->>PG: INSERT INTO users (email, bcrypt_hash, 'local')
    else User found (Login)
        Auth->>Auth: bcrypt.checkpw(password, hash)
    end
    Auth->>Auth: jwt.encode(payload, RS256_PRIVATE_KEY)
    Auth->>RD: SET refresh_token:{token} = email  TTL=30d
    Auth-->>GW: { access_token } + Set-Cookie: refresh_token (HttpOnly)
    GW-->>C: 200 OK

    Note over C, GW: Path B — Google OIDC (POST /auth/google/callback)
    C->>GW: POST /auth/google/callback { id_token OR code }
    GW->>Auth: Forward request
    Auth->>Auth: base64url.decode(id_token.payload) → { email, sub }
    Auth->>PG: SELECT WHERE google_sub=$1 OR email=$2
    alt New Google user (auto-provision)
        Auth->>PG: INSERT INTO users (email, 'GOOGLE_OIDC_USER_NO_PASSWORD', 'google_oidc', sub)
    end
    Auth->>Auth: jwt.encode(payload, RS256_PRIVATE_KEY)
    Auth->>RD: SET refresh_token:{token} = email  TTL=30d
    Auth-->>C: { access_token } + Set-Cookie: refresh_token (HttpOnly)
```

### Auth Flow: Protected Request Verification (Zero-Trust Gateway)

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant GW as API Gateway<br/>(RS256 Public Key — in-memory)
    participant SVC as Internal Service<br/>(Shortener / Analytics)

    C->>GW: GET /api/v1/... Authorization: Bearer <JWT>
    GW->>GW: jwt.decode(token, RS256_PUBLIC_KEY)<br/>★ No network call to Auth Service
    alt Valid token
        GW->>SVC: Forward request (internal Docker network)
        SVC-->>GW: Response
        GW-->>C: 200 OK
    else Expired / Invalid signature
        GW-->>C: 401 Unauthorized
    end
```

### Auth Flow: Refresh Token Rotation

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant GW as API Gateway
    participant Auth as Auth Service
    participant RD as Redis

    Note over C: Access token expired (15 min TTL)
    C->>GW: POST /auth/refresh  Cookie: refresh_token=<opaque>
    GW->>Auth: Forward cookies
    Auth->>RD: GET refresh_token:{token}  → email
    alt Token found (valid, not revoked)
        Auth->>Auth: jwt.encode(new payload, RS256_PRIVATE_KEY)
        Auth-->>C: { access_token } (new 15-min JWT)
    else Token missing or expired (30d TTL elapsed)
        Auth-->>C: 401 — re-login required
    end

    Note over C: Logout
    C->>GW: POST /auth/logout  Cookie: refresh_token=<opaque>
    GW->>Auth: Forward cookies
    Auth->>RD: DEL refresh_token:{token}  ★ Immediate revocation
    Auth-->>C: 200 + Set-Cookie: refresh_token="" (cleared)
```

---

## 2. Auth Service Implementation

### Core Modules

| Module | Responsibility |
|--------|---------------|
| `services/auth/main.py` | FastAPI app — login, refresh, logout, Google OIDC endpoints |
| `services/auth/tokens.py` | `create_access_token()` (RS256 JWT), `generate_refresh_token()` (opaque), Redis store/get/delete |
| `services/auth/passwords.py` | `hash_password()` (bcrypt), `verify_password()` (bcrypt.checkpw) |
| `services/auth/oidc.py` | `build_google_auth_url()`, `exchange_code_for_id_token()`, `parse_google_id_token()` |
| `services/auth/database.py` | asyncpg connection pool, `init_users_table()` (auto-migration) |
| `services/auth/config.py` | ENV-driven config: RSA keys, Redis URL, OIDC credentials, TTL constants |
| `services/gateway/main.py` | `verify_token()` — pure in-memory RS256 public key JWT decode (no auth service call) |

### JWT Payload Schema

```json
{
  "sub": "user@example.com",
  "email": "user@example.com",
  "sso_provider": "local | google_oidc",
  "iat": 1700000000,
  "exp": 1700000900
}
```

### PostgreSQL `users` Table Schema

```sql
CREATE TABLE IF NOT EXISTS users (
    email         VARCHAR(255) PRIMARY KEY,
    password_hash VARCHAR(255) NOT NULL,
    sso_provider  VARCHAR(50)  DEFAULT 'local',
    google_sub    VARCHAR(255),
    created_at    TIMESTAMPTZ  DEFAULT NOW()
);
```

### Redis Refresh Token Schema

```
Key:   refresh_token:{64-byte-urlsafe-random-token}
Value: user@example.com
TTL:   2,592,000 seconds (30 days)
```

---

## 3. System Architecture

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
            GoogleH["GET  /auth/google/login<br/>POST /auth/google/callback"]
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
```

---

## 4. Repository Structure

```text
Microservice-Auth-Platform/
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
│   ├── auth/         # ★ JWT auth, Google OIDC & user management (core)
│   ├── gateway/      # Reverse proxy & zero-trust RS256 JWT verification
│   └── shortener/    # URL encoding, decoding & cache layer
├── tests/            # Automated test suites
│   ├── e2e/          # End-to-end user journey tests
│   ├── integration/  # Inter-service integration tests (Redis token store)
│   └── unit/         # Unit tests — bcrypt, RS256 JWT, Google OIDC
├── .dockerignore
├── .gitignore
├── pyproject.toml    # Dependency management & pytest config
└── uv.lock           # Locked dependency lockfile
```

---

## 5. Running Tests

```bash
# Unit tests (offline — no Docker required)
uv run pytest tests/unit/ -v

# Integration tests (requires Redis)
uv run pytest tests/integration/ -v

# E2E tests (requires full Docker stack)
docker compose up -d
uv run pytest tests/e2e/ -v
```
