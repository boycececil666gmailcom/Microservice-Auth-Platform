"""
API Gateway — thin transparent proxy.

Responsibilities:
  - Receive HTTP requests from the end user
  - Validate JWT access tokens locally using the RS256 public key
  - Forward the request to the appropriate internal service via httpx
  - Return the response to the end user

No business logic lives here.
"""

import os
from contextlib import asynccontextmanager

import httpx
import jwt
from fastapi import FastAPI, HTTPException, Request, Response, Depends


# ── Internal service URLs ─────────────────────────────────────────────────────
# These are Docker internal hostnames — not exposed to the outside world.
SHORTENER_URL = os.environ.get("SHORTENER_URL", "http://shortener:8001")
AUTH_URL = os.environ.get("AUTH_URL", "http://auth:8002")
ANALYTICS_URL = os.environ.get("ANALYTICS_URL", "http://analytics:8003")

# ── RS256 Public Key (used to VERIFY tokens — cannot sign) ────────────────────
JWT_ALGORITHM = "RS256"
JWT_PUBLIC_KEY = os.environ.get("JWT_PUBLIC_KEY")

if not JWT_PUBLIC_KEY:
    raise RuntimeError("Gateway requires JWT_PUBLIC_KEY env var")


# ── Shared async httpx client ─────────────────────────────────────────────────
# A single client is reused across requests (connection pool).
_http_client: httpx.AsyncClient | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Create the shared HTTP client on startup; close it on shutdown."""
    global _http_client
    _http_client = httpx.AsyncClient(timeout=10.0)
    yield
    await _http_client.aclose()


def get_client() -> httpx.AsyncClient:
    if _http_client is None:
        raise RuntimeError("HTTP client not initialised")
    return _http_client


# ── App ───────────────────────────────────────────────────────────────────────

app = FastAPI(title="API Gateway", version="0.1.0", lifespan=lifespan)

async def verify_token(request: Request) -> dict:
    """Verify JWT access token locally using the RS256 public key.

    No network call to the Auth Service — pure in-memory cryptographic check.
    """
    auth_header = request.headers.get("Authorization")
    if not auth_header or not auth_header.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Not authenticated")

    token = auth_header.split(" ", 1)[1]
    try:
        payload = jwt.decode(token, JWT_PUBLIC_KEY, algorithms=[JWT_ALGORITHM])
        return payload
    except jwt.PyJWTError:
        raise HTTPException(status_code=401, detail="Invalid or expired token")


# ── Health ────────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    return {"status": "ok"}


# ── Write Path ────────────────────────────────────────────────────────────────

@app.post("/api/v1/shorten", status_code=201)
async def shorten_url(request: Request, token: dict = Depends(verify_token)):
    """Forward POST /api/v1/shorten to the Shortener service."""
    body = await request.body()
    headers = {"Content-Type": "application/json"}

    resp = await get_client().post(
        f"{SHORTENER_URL}/shorten",
        content=body,
        headers=headers,
    )

    _raise_for_upstream_error(resp)
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
    )


# ── Read Path ─────────────────────────────────────────────────────────────────

@app.get("/api/v1/urls/{short_url}")
async def get_url(short_url: int, token: dict = Depends(verify_token)):
    """Forward GET /api/v1/urls/{short_url} to the Shortener service."""
    resp = await get_client().get(f"{SHORTENER_URL}/urls/{short_url}")
    _raise_for_upstream_error(resp)
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
    )


@app.get("/api/v1/analytics/stats")
async def get_analytics_stats(token: dict = Depends(verify_token)):
    """Forward GET /api/v1/analytics/stats to the Analytics service."""
    resp = await get_client().get(f"{ANALYTICS_URL}/stats")
    _raise_for_upstream_error(resp)
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
    )


@app.get("/r/{short_url}")
async def redirect(short_url: int):
    """Forward redirect requests to the Shortener service."""
    resp = await get_client().get(
        f"{SHORTENER_URL}/r/{short_url}",
        follow_redirects=False,  # let the 302 pass back to the browser as-is
    )
    _raise_for_upstream_error(resp)
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        headers=dict(resp.headers),
    )


# ── Auth Routes ───────────────────────────────────────────────────────────────

@app.post("/auth/login")
async def auth_login(request: Request):
    """Forward POST /auth/login to the Auth service."""
    body = await request.body()
    resp = await get_client().post(
        f"{AUTH_URL}/auth/login",
        content=body,
        headers={"Content-Type": "application/json"},
    )
    _raise_for_upstream_error(resp)
    # Forward the response including Set-Cookie headers from the auth service.
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
        headers=dict(resp.headers),
    )


@app.post("/auth/refresh")
async def auth_refresh(request: Request):
    """Forward POST /auth/refresh to the Auth service."""
    # Pass cookies through so the auth service can read refresh_token.
    cookies = request.cookies
    resp = await get_client().post(
        f"{AUTH_URL}/auth/refresh",
        cookies=cookies,
    )
    _raise_for_upstream_error(resp)
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
    )


@app.post("/auth/logout")
async def auth_logout(request: Request):
    """Forward POST /auth/logout to the Auth service."""
    cookies = request.cookies
    resp = await get_client().post(
        f"{AUTH_URL}/auth/logout",
        cookies=cookies,
    )
    _raise_for_upstream_error(resp)
    # Forward the response including Set-Cookie (clear cookie) headers.
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type="application/json",
        headers=dict(resp.headers),
    )


# ── Helpers ───────────────────────────────────────────────────────────────────

def _raise_for_upstream_error(resp: httpx.Response) -> None:
    """Re-raise 4xx/5xx responses from upstream as FastAPI HTTPExceptions."""
    if resp.status_code >= 400:
        try:
            detail = resp.json().get("detail", resp.text)
        except Exception:
            detail = resp.text
        raise HTTPException(status_code=resp.status_code, detail=detail)
