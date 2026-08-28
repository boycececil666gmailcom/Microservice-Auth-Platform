# Auth Service Design

This document details the authentication flows using the **Short-Lived Access Token + Long-Lived Refresh Token** pattern with a **Single Universal RS256 JWT Schema** (`sub = email`) for both Password and Google OIDC authentication.

## 1a. Email Password Sign Up / Login & Token Issuance Flow

```mermaid
sequenceDiagram
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Auth Service
    participant PG as User DB (Postgres)
    participant R as Token DB (Redis)

    C->>G: POST /auth/login {email, password}
    G->>A: Forward request
    A->>PG: Fetch user record by email
    
    alt User DOES NOT exist (Sign Up)
        A->>A: Hash password with bcrypt (SLOW)
        A->>PG: INSERT INTO users {email, password_hash, sso_provider='local'}
        PG-->>A: Return user record (email)
    else User DOES exist (Login)
        PG-->>A: Return user record (password_hash & sso_provider)
        A->>A: Verify password against Hash (bcrypt)
        alt Password Incorrect
            A-->>G: 401 Unauthorized
            G-->>C: 401 Unauthorized (Stop)
        end
    end
    
    A->>A: Generate Universal RS256 JWT (sub: email, email: email, sso_provider='local', exp: 15m)
    A->>A: Generate Refresh Token (Opaque 48-char string)
    
    A->>R: SET refresh_token:{token} = email EX 30d
    
    A-->>G: 200 OK
    Note over A,G: Access Token (JSON body)<br/>Refresh Token (Set-Cookie: HttpOnly)
    G-->>C: 200 OK
```

---

## 1b. Google OpenID Connect (OIDC) Authorization Code Flow

```mermaid
sequenceDiagram
    autonumber
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Auth Service
    participant GO as Google OIDC Provider
    participant PG as User DB (Postgres)
    participant R as Session Store (Redis)

    Note over C,GO: Phase 1: Start Authorization Code Flow
    C->>G: GET /auth/google/login
    G->>A: Forward request
    A->>A: Generate state token (client must validate on callback)
    A-->>G: { auth_url, state }
    G-->>C: { auth_url, state }

    Note over C,GO: Phase 2: Google Authentication and Consent
    C->>GO: Open authorization URL<br/>scope: openid email profile
    alt User authenticates and grants consent
        rect rgb(235, 247, 238)
            GO-->>C: Redirect with authorization code and state
        end
    else User denies consent or authentication fails
        rect rgb(253, 237, 237)
            GO-->>C: Redirect with OAuth error
        end
    end

    Note over C,GO: Phase 3: Callback and Google Token Exchange
    C->>G: POST /auth/google/callback { code, state }
    G->>A: Forward callback request
    A->>GO: POST /token { code, client credentials, redirect_uri }
    alt Code exchange succeeds
        rect rgb(235, 247, 238)
            GO-->>A: Google ID Token (JWT)
        end
    else Code is invalid or expired
        rect rgb(253, 237, 237)
            GO-->>A: OAuth error
            A-->>C: 400 Google Code Exchange failed
        end
    end

    Note over A,R: Phase 4: Resolve Identity and Issue Platform Tokens
    A->>A: Parse Google claims (email, sub, name, picture)
    A->>PG: Fetch user by google_sub OR email

    alt User DOES NOT exist
        rect rgb(235, 247, 238)
            A->>PG: Create user {email, provider=google_oidc, google_sub}
        end
    else User DOES exist
        rect rgb(235, 247, 238)
            PG-->>A: Existing user identity
        end
    end

    A->>A: Sign platform access JWT with RS256 private key (15m)
    A->>A: Generate opaque refresh token
    A->>R: SET refresh_token:{opaque token} = email (TTL 30d)

    A-->>G: Access JWT (JSON) + refresh token (HttpOnly cookie)
    G-->>C: 200 OK
    Note over C,A: Google ID Token proves the Google identity to Auth Service
    Note over A,R: Platform access JWT authorizes calls to internal APIs
```

---

## 2. Standard API Request & Background Refresh Flow

This diagram shows what happens during a standard API request. If the access token is expired, the client automatically handles it by using the refresh token in the background to get a new access token, and then retries the original request.

```mermaid
sequenceDiagram
    participant C as Client (Browser)
    participant G as API Gateway
    participant S as Shortener Service
    participant A as Auth Service
    participant PG as User DB (Postgres)
    participant R as Token DB (Redis)

    Note over C,S: --- 1. Standard Request Attempt ---
    C->>G: POST /api/v1/shorten
    Note right of C: Header: Authorization: Bearer <Universal_JWT>
    
    G->>G: Verify RS256 JWT Signature (in-memory using RSA Public Key)
    
    alt Token is Valid
        G->>S: Forward to Shortener
        Note right of G: Add Header: X-User-ID: <sub_from_jwt (email)>
        S-->>G: 201 Created
        G-->>C: 201 Created (Success!)
        
    else Token is Expired
        G-->>C: 401 Unauthorized
        
        Note over C,R: --- 2. Automatic Background Refresh ---
        C->>G: POST /auth/refresh
        Note right of C: Cookie: refresh_token=[opaque string]
        
        G->>A: Forward request
        A->>R: GET refresh_token:{token}
        
        alt Refresh Token Invalid / Expired
            R-->>A: Null
            A-->>G: 401 Unauthorized
            G-->>C: 401 Unauthorized (Must login again)
            
        else Refresh Token Valid
            R-->>A: Return email
            A->>PG: Check if email exists in DB
            PG-->>A: User exists & Active
            A->>A: Generate NEW Universal RS256 JWT (sub: email, email: email, sso_provider, 15m)
            A-->>G: 200 OK {access_token: "..."}
            G-->>C: 200 OK
            
            Note over C,S: --- 3. Retry Original Request ---
            C->>G: POST /api/v1/shorten (retry)
            Note right of C: Header: Authorization: Bearer <NEW_UNIVERSAL_JWT>
            G->>G: Verify NEW JWT Signature
            G->>S: Forward to Shortener
            S-->>G: 201 Created
            G-->>C: 201 Created (Success!)
        end
    end
```

---

## 3. Logout / Revocation Flow

When the user logs out, we immediately delete the Refresh Token key from Redis.

```mermaid
sequenceDiagram
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Auth Service
    participant R as Token DB (Redis)

    C->>G: POST /auth/logout
    Note right of C: Cookie: refresh_token=[opaque string]
    
    G->>A: Forward request
    A->>R: DEL refresh_token:{token}
    
    A-->>G: 200 OK (Clear Cookie)
    G-->>C: 200 OK (User logged out)
    
    Note over C,R: The user's Access Token might still work<br/>for up to 15 minutes (ghost window), but<br/>they cannot refresh it anymore.
```
