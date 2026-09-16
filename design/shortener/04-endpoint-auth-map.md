# Endpoint Authentication & Routing Map

This document outlines the authentication requirements for all endpoints exposed by the API Gateway and details how requests are routed to downstream services.

## 1. Routing & Authentication Architecture

The Gateway acts as the single entrypoint for external clients. It enforces JWT verification for versioned business resource routes while bypassing verification for public assets, redirects, and session management routes.

```
                         ┌─────────────────────────┐
                         │   Incoming HTTP Request │
                         └────────────┬────────────┘
                                      │
                                      ▼
                      ┌───────────────────────────────┐
                      │      API Gateway (8000)       │
                      └──────────────┬────────────────┘
                                     │
             ┌───────────────────────┴───────────────────────┐
             │                                               │
             ▼                                               ▼
  [ JWT AUTHENTICATED ]                           [ NO JWT REQUIRED ]
     (Protected API)                            (Public & Session API)
             │                                               │
   ┌─────────┼─────────┐                           ┌─────────┼─────────┬─────────┐
   │         │         │                           │         │         │         │
   ▼         ▼         ▼                           ▼         ▼         ▼         ▼
 POST       GET       GET                         POST      POST      POST      GET
/api/v1/  /api/v1/  /api/v1/                     /auth/    /auth/    /auth/     /r/
shorten   urls/{id} analytics                    login     refresh   logout    {id}
   │         │         │                           │         │         │         │
   ▼         ▼         ▼                           ▼         ▼         ▼         ▼
[Shortener] [Shortener] [Analytics]               [Auth]    [Auth]    [Auth]  [Shortener]
 (:8001)   (:8001)     (:8003)                   (:8002)   (:8002)   (:8002)   (:8001)
```

## 2. Endpoint Specifications

| Method | Endpoint Path | Auth Type | Target Upstream Service | Description |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/health` | None | Gateway Internal | Performs Gateway health verification. |
| **GET** | `/.well-known/jwks.json` | None | `http://auth:8002/.well-known/jwks.json` | Publishes the current RSA verification key for gateway JWT validation. |
| **POST** | `/auth/login` | None | `http://auth:8002/auth/login` | Registers or authenticates a user; returns a JWT access token in the response body and stores a refresh token in an `HttpOnly` cookie. |
| **POST** | `/auth/refresh` | Refresh Cookie | `http://auth:8002/auth/refresh` | Consumes the `refresh_token` cookie to issue a new short-lived JWT access token. |
| **POST** | `/auth/logout` | Refresh Cookie | `http://auth:8002/auth/logout` | Revokes the active refresh token and clears the client's cookie. |
| **POST** | `/api/v1/shorten` | JWT Bearer | `http://shortener:8001/shorten` | Generates a shortened URL representation of the supplied destination URL. |
| **GET** | `/api/v1/urls/{short_url}` | JWT Bearer | `http://shortener:8001/urls/{short_url}` | Retrieves the original target URL and creation metadata for a short URL ID. |
| **GET** | `/api/v1/analytics/stats` | JWT Bearer | `http://analytics:8003/stats` | Returns system-wide statistics and redirect frequency per short URL. |
| **GET** | `/r/{short_url}` | None | `http://shortener:8001/r/{short_url}` | Public redirect endpoint forwarding user requests to the destination URL. |
