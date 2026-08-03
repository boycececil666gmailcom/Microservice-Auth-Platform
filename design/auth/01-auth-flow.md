# Auth Service Design

This document details the authentication flows using the **Short-Lived Access Token + Long-Lived Refresh Token** pattern for both Password and Google OIDC authentication.

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
        A->>PG: INSERT INTO users {username=email, password_hash, sso_provider='local'}
        PG-->>A: Return new user_id
    else User DOES exist (Login)
        PG-->>A: Return existing User Hash & sso_provider
        A->>A: Verify password against Hash (SLOW)
        alt Password Incorrect
            A-->>G: 401 Unauthorized
            G-->>C: 401 Unauthorized (Stop)
        end
    end
    
    A->>A: Generate Consolidated Access Token (JWT sub, email, sso_provider='local', exp: 15m)
    A->>A: Generate Refresh Token (Opaque string)
    
    A->>R: SET refresh_token:{token} = user_id EX 30d
    
    A-->>G: 200 OK
    Note over A,G: Access Token (JSON body)<br/>Refresh Token (Set-Cookie: HttpOnly)
    G-->>C: 200 OK
```

---

## 1b. Google OpenID Connect (OIDC) Authorization Code Flow

```mermaid
sequenceDiagram
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Auth Service
    participant GO as Google OIDC Endpoint
    participant PG as User DB (Postgres)
    participant R as Token DB (Redis)

    C->>G: GET /auth/google/login
    G->>A: Forward request
    A->>A: Generate state token for CSRF protection
    A-->>G: Return {auth_url, state}
    G-->>C: Return {auth_url, state}

    Note over C,GO: --- Google Authentication & Consent ---
    C->>GO: Redirect browser to Google Authorization URL
    GO-->>C: User authenticates & redirects to /auth/google/callback?code=CODE&state=STATE

    Note over C,R: --- Callback & Token Exchange ---
    C->>G: POST /auth/google/callback {code, state}
    G->>A: Forward callback request
    A->>GO: POST /token (Exchange code for Google ID Token)
    GO-->>A: Return Google ID Token (JWT)

    A->>A: Parse claims (email, google_sub)
    A->>PG: Fetch user by google_sub OR email

    alt User DOES NOT exist
        A->>PG: INSERT INTO users {username=email, sso_provider='google_oidc', google_sub}
        PG-->>A: Return user record (email)
    else User DOES exist
        PG-->>A: Return user record (email)
    end

    A->>A: Generate Consolidated Access Token (JWT sub, email, sso_provider='google_oidc', exp: 15m)
    A->>A: Generate Refresh Token (Opaque string)
    A->>R: SET refresh_token:{token} = user_id EX 30d

    A-->>G: 200 OK
    Note over A,G: Consolidated RS256 JWT Access Token (JSON body)<br/>Refresh Token (Set-Cookie: HttpOnly)
    G-->>C: 200 OK
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
    Note right of C: Header: Authorization: Bearer <Consolidated_JWT>
    
    G->>G: Verify JWT Signature (using RSA Public Key)
    
    alt Token is Valid
        G->>S: Forward to Shortener
        Note right of G: Add Header: X-User-ID: <sub_from_jwt>
        S-->>G: 201 Created
        G-->>C: 201 Created (Success!)
        
    else Token is Expired
        G-->>C: 401 Unauthorized
        
        Note over C,R: --- 2. Automatic Background Refresh ---
        C->>G: POST /auth/refresh
        Note right of C: Cookie: refresh_token=<opaque_string>
        
        G->>A: Forward request
        A->>R: GET refresh_token:{token}
        
        alt Refresh Token Invalid / Expired
            R-->>A: Null
            A-->>G: 401 Unauthorized
            G-->>C: 401 Unauthorized (Must login again)
            
        else Refresh Token Valid
            R-->>A: Return user_id
            A->>PG: Fetch user email & sso_provider by user_id
            PG-->>A: Return user details (Active)
            A->>A: Generate NEW Consolidated Access Token (JWT sub, email, sso_provider, 15m)
            A-->>G: 200 OK {access_token: "..."}
            G-->>C: 200 OK
            
            Note over C,S: --- 3. Retry Original Request ---
            C->>G: POST /api/v1/shorten (retry)
            Note right of C: Header: Authorization: Bearer <NEW_CONSOLIDATED_JWT>
            G->>G: Verify NEW JWT Signature
            G->>S: Forward to Shortener
            S-->>G: 201 Created
            G-->>C: 201 Created (Success!)
        end
    end
```

---

## 3. Logout / Revocation Flow

When the user logs out, we immediately delete the Refresh Token from Redis.

```mermaid
sequenceDiagram
    participant C as Client (Browser)
    participant G as API Gateway
    participant A as Auth Service
    participant R as Token DB (Redis)

    C->>G: POST /auth/logout
    Note right of C: Cookie: refresh_token=<opaque_string>
    
    G->>A: Forward request
    A->>R: DEL refresh_token:{token}
    
    A-->>G: 200 OK (Clear Cookie)
    G-->>C: 200 OK (User logged out)
    
    Note over C,R: The user's Access Token might still work<br/>for up to 15 minutes (ghost window), but<br/>they cannot refresh it anymore.
```
