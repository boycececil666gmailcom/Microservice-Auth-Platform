# Microservice Authentication Platform

> Production-oriented reference platform for identity, session management, protected URL shortening, and event-driven analytics using decoupled services.

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-RS256-000000?style=flat&logo=jsonwebtokens&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7.0+-DC382D?style=flat&logo=redis&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat&logo=postgresql&logoColor=white)
![Apache Kafka](https://img.shields.io/badge/Apache_Kafka-3.0+-231F20?style=flat&logo=apachekafka&logoColor=white)
![Terraform](https://img.shields.io/badge/Terraform-1.5+-844FBA?style=flat&logo=terraform&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker&logoColor=white)

---

## 1. Core Purpose & Business Value

This platform delivers a complete user identity and session management foundation, enabling product teams to ship authenticated features faster while giving security teams full control over who accesses what — and for how long.

- **Frictionless Onboarding & Social Sign-in**: Users can register or sign in using their existing Google accounts with a single click, dramatically reducing sign-up abandonment and increasing conversion rates for consumer-facing products.
- **Session Revocation with a Bounded Access Window**: Refresh sessions can be revoked immediately; already-issued access tokens remain valid only for their configured short lifetime (15 minutes by default).
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
        APIGW["API Gateway<br/>(Envoy)<br/>RS256 Public Key Verification"]
    end

    subgraph ReadPath["Read Path"]
        Redirect["API Gateway<br/>(Envoy)<br/>RS256 Public Key Verification"]
    end

    subgraph AuthSvc["Auth Service"]
        Auth["Auth Handler<br/>(RS256 Private Key Signing)<br/>Google OIDC + Local Auth"]
        subgraph AuthDB["Owned Storage"]
            UserDB[("User DB<br/>(PostgreSQL)")]
            AuthRedis["Session Store<br/>(Redis)<br/>refresh_token:{SHA-256(token)} TTL 30d"]
        end
    end

    subgraph ShortenerSvc["Shortener Service"]
        Shortener["Shortener Handler<br/>(Go net/http)"]
        subgraph ShortenerDB["Owned Storage"]
            Redis["Cache<br/>(Redis)<br/>url:{id} TTL 24h"]
            Primary[("Primary DB<br/>(PostgreSQL)")]
        end
    end

    subgraph Async
        Queue["Queue<br/>(Apache Kafka)"]
        Analytics["Analytics Service<br/>(Go net/http)"]
    end

    User --> APIGW
    APIGW --> Auth
    APIGW --> Shortener

    Auth --> UserDB
    Auth --> AuthRedis

    Shortener --> Redis
    Shortener --> Primary
    Redis -. Cache Miss .-> Primary

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
        ExternalClient["curl / Browser / Go tests"]
    end

    subgraph Exposed["Exposed to Host"]
        GW["gateway<br/>(Envoy)<br/>RS256 Public Key JWT Verification<br/>port 8000"]
    end

    subgraph Internal["Docker Internal Network - not reachable from host"]

        subgraph ShortenerCtr["shortener (Go, port 8001)"]
            direction TB
            WriteH["POST /shorten"]
            ReadH["GET /urls/:id<br/>GET /r/:id"]
        end

        subgraph AuthCtr["auth (Go, port 8002)"]
            direction TB
            LoginH["POST /auth/login"]
            RefreshH["POST /auth/refresh"]
            LogoutH["POST /auth/logout"]
            GoogleCbH["POST /auth/google/callback"]
        end

        subgraph AnalyticsCtr["analytics (Go, port 8003)"]
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
            TokenStore["key: refresh_token:{SHA-256(token)}<br/>value: email + provider<br/>TTL: 30d"]
        end

        subgraph AuthDBCtr["auth-db (PostgreSQL 16, port 5432)"]
            UserPG[("table: users")]
        end

    end

    ExternalClient -->|"port 8000 - only exposed port"| GW
    GW -->|"Envoy proxy - internal network only"| ShortenerCtr
    GW -->|"Envoy proxy - internal network only"| AuthCtr
    GW -->|"Envoy proxy - internal network only"| AnalyticsCtr
    WriteH -->|"INSERT ON CONFLICT"| PG
    ReadH -->|"GET url:{id}"| Cache
    Cache -.->|"Cache MISS"| PG
    PG -.->|"Cache WARM"| Cache
    ReadH -->|"Publish event (async)"| KafkaCtr
    KafkaCtr -.->|"Consume event"| ConsumeH
    LoginH -->|"SELECT / INSERT"| UserPG
    LoginH -->|"SET token digest"| TokenStore
    RefreshH -->|"Atomically rotate token"| TokenStore
    LogoutH -->|"DEL token digest"| TokenStore
    GoogleCbH -->|"SELECT / INSERT"| UserPG
    GoogleCbH -->|"SET token digest"| TokenStore
```

---

## 3. Repository Structure

```text
Microservice-Auth-Platform/
├── cmd/              # Service entry points
│   ├── analytics/
│   ├── auth/
│   └── shortener/
├── internal/         # Go service implementations and unit tests
│   ├── analytics/    # Kafka consumer and in-memory counters
│   ├── auth/         # JWT, OIDC, bcrypt, PostgreSQL, and Redis
│   └── shortener/    # PostgreSQL, Redis cache, redirects, and Kafka producer
├── services/         # Multi-stage Dockerfiles for the three Go binaries
├── infra_tf/         # Kubernetes infrastructure as Terraform
├── design/           # Architecture and API design documents
├── tests/e2e/        # Go end-to-end tests (build tag: e2e)
├── go.mod
└── go.sum
```

## 4. Build and Test

Go 1.26 or newer is required.

```bash
go test ./...
go build ./cmd/...
```

Run the end-to-end suite against a deployed gateway with:

```bash
GATEWAY_URL=http://localhost:8000 go test -tags=e2e ./tests/e2e/ -v
```

To include the mock Google flow, deploy with `allow_mock_oidc = true`, use a `mock-*` client ID, and set `RUN_MOCK_OIDC_E2E=true` for the test command.

## 5. Security and Deployment Notes

- The Auth service requires an RSA PKCS#8 private key through `JWT_PRIVATE_KEY` or `RSA_PRIVATE_KEY_PEM`; no development signing key is embedded in the binary.
- Envoy obtains the matching public key from the Auth service's JWKS endpoint and caches it for JWT verification.
- Envoy applies a bounded local token-bucket rate limit before JWT processing; production multi-replica deployments should use a shared global rate-limit service at the edge.
- Real Google ID tokens are verified for their RSA signature, issuer, audience, expiry, authorized party, and verified email. Mock OIDC additionally requires `ALLOW_MOCK_OIDC=true` and a `mock-*` Google client ID.
- Refresh tokens are 48 random bytes, stored only by SHA-256 digest, rotated after every refresh, and delivered in an `HttpOnly`, `SameSite=Lax` cookie. Set `cookie_secure = true` for HTTPS deployments.
- `infra_tf/kafka.tf` provides a single-node Kafka broker for local/test clusters. Use a managed or multi-node Kafka installation in production.

Create `infra_tf/terraform.tfvars` from the example, supply strong database passwords and an RSA private key, then run the scripts in `CICD_script/` in numerical order.
