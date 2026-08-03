"""
Auth Service — handles user registration, password login, token refresh, logout, and Google OIDC.

Endpoints:
  POST /auth/login           — Sign up (if user doesn't exist) or login with password.
  POST /auth/refresh         — Exchange a refresh token for a new access token.
  POST /auth/logout          — Revoke the refresh token.
  GET  /auth/google/login    — Generate Google OAuth 2.0 authorization URL.
  POST /auth/google/callback — Exchange Google ID token / code for RS256 JWT & refresh cookie.
"""

from contextlib import asynccontextmanager

import asyncpg
from fastapi import Cookie, Depends, FastAPI, HTTPException, Response
from . import database, oidc
from .database import close_pool, create_db_pool, get_db
from .passwords import hash_password, verify_password
from .schemas import GoogleCallbackRequest, LoginRequest, TokenResponse
from .tokens import (
    close_redis_pool,
    create_access_token,
    create_redis_pool,
    delete_refresh_token,
    generate_refresh_token,
    get_user_id_by_token,
    store_refresh_token,
    REFRESH_TOKEN_TTL_SECONDS,
)


#region Lifespan & App Setup
@asynccontextmanager
async def lifespan(app: FastAPI):
    """Open DB/Redis pools and ensure the users table and Google OIDC columns exist."""
    await create_db_pool()
    await create_redis_pool()

    # Auto-create the users table and SSO columns if they don't exist yet.
    async with database.pool.acquire() as conn:
        await conn.execute("""
            CREATE TABLE IF NOT EXISTS users (
                id            SERIAL       PRIMARY KEY,
                username      VARCHAR(50)  UNIQUE NOT NULL,
                password_hash VARCHAR(255) NOT NULL,
                sso_provider  VARCHAR(50)  DEFAULT 'local',
                google_sub    VARCHAR(255),
                created_at    TIMESTAMPTZ  DEFAULT NOW()
            )
        """)
        await conn.execute("ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_provider VARCHAR(50) DEFAULT 'local'")
        await conn.execute("ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub VARCHAR(255)")

    yield

    await close_pool()
    await close_redis_pool()


app = FastAPI(title="Auth Service", version="0.1.0", lifespan=lifespan)
#endregion


#region Health Endpoints
@app.get("/health")
async def health():
    return {"status": "ok"}
#endregion


#region Password Authentication Routes
@app.post("/auth/login", response_model=TokenResponse)
async def login(
    body: LoginRequest,
    response: Response,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Sign up if the user doesn't exist, or log in if they do.

    Returns an access token in the JSON body and sets the refresh token
    as an HttpOnly cookie.
    """
    row = await conn.fetchrow(
        "SELECT id, password_hash FROM users WHERE username = $1",
        body.user,
    )

    if row is None:
        # Sign Up
        hashed = hash_password(body.password)
        user_id = await conn.fetchval(
            "INSERT INTO users (username, password_hash, sso_provider) VALUES ($1, $2, 'local') RETURNING id",
            body.user,
            hashed,
        )
    else:
        # Login
        if not verify_password(body.password, row["password_hash"]):
            raise HTTPException(status_code=401, detail="Invalid credentials")
        user_id = row["id"]

    access_token = create_access_token(user_id)
    refresh_token = generate_refresh_token()

    await store_refresh_token(refresh_token, user_id)

    response.set_cookie(
        key="refresh_token",
        value=refresh_token,
        httponly=True,
        secure=False,
        samesite="lax",
        max_age=REFRESH_TOKEN_TTL_SECONDS,
        path="/",
    )

    return TokenResponse(access_token=access_token)


@app.post("/auth/refresh", response_model=TokenResponse)
async def refresh(
    refresh_token: str | None = Cookie(default=None),
    conn: asyncpg.Connection = Depends(get_db),
):
    """Exchange a valid refresh token (from cookie) for a new access token."""
    if refresh_token is None:
        raise HTTPException(status_code=401, detail="Missing refresh token")

    user_id = await get_user_id_by_token(refresh_token)
    if user_id is None:
        raise HTTPException(status_code=401, detail="Invalid or expired refresh token")

    exists = await conn.fetchval(
        "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)",
        user_id,
    )
    if not exists:
        raise HTTPException(status_code=401, detail="User no longer exists")

    access_token = create_access_token(user_id)
    return TokenResponse(access_token=access_token)


@app.post("/auth/logout")
async def logout(
    response: Response,
    refresh_token: str | None = Cookie(default=None),
):
    """Revoke the refresh token and clear the cookie."""
    if refresh_token:
        await delete_refresh_token(refresh_token)

    response.delete_cookie(key="refresh_token", path="/")
    return {"detail": "Logged out"}
#endregion


#region Google OpenID Connect (OIDC) Endpoints
@app.get("/auth/google/login")
async def google_login():
    """Return the Google OAuth 2.0 / OIDC authorization redirect URL."""
    auth_url = oidc.build_google_auth_url()
    return {"auth_url": auth_url}


@app.post("/auth/google/callback", response_model=TokenResponse)
async def google_callback(
    body: GoogleCallbackRequest,
    response: Response,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Google OIDC Callback — processes Google ID Token (Sign in with Google).

    Auto-provisions Google user if not registered, and issues signed RS256 JWT
    access token and refresh token cookie (identical to /auth/login).
    """
    try:
        user_info = oidc.parse_google_id_token(body.id_token)
    except ValueError as err:
        raise HTTPException(status_code=400, detail=f"Invalid Google ID Token: {err}")

    email = user_info["email"]
    google_sub = user_info["google_sub"]

    # 1. Resolve or auto-provision user by Google Sub or Email.
    row = await conn.fetchrow(
        "SELECT id FROM users WHERE google_sub = $1 OR username = $2",
        google_sub,
        email,
    )

    if row is None:
        user_id = await conn.fetchval(
            """INSERT INTO users (username, password_hash, sso_provider, google_sub)
               VALUES ($1, 'GOOGLE_OIDC_USER_NO_PASSWORD', 'google_oidc', $2) RETURNING id""",
            email,
            google_sub,
        )
    else:
        user_id = row["id"]

    # 2. Generate identical RS256 JWT Access Token and Refresh Token Cookie.
    access_token = create_access_token(user_id)
    refresh_token = generate_refresh_token()

    await store_refresh_token(refresh_token, user_id)

    response.set_cookie(
        key="refresh_token",
        value=refresh_token,
        httponly=True,
        secure=False,
        samesite="lax",
        max_age=REFRESH_TOKEN_TTL_SECONDS,
        path="/",
    )

    return TokenResponse(access_token=access_token)
#endregion
