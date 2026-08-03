"""
Auth Service — handles user registration, login, token refresh, and logout.

Endpoints:
  POST /auth/login    — Sign up (if user doesn't exist) or login.
  POST /auth/refresh  — Exchange a refresh token for a new access token.
  POST /auth/logout   — Revoke the refresh token.
"""

from contextlib import asynccontextmanager

import asyncpg
from fastapi import Cookie, Depends, FastAPI, HTTPException, Response
from pydantic import BaseModel

from . import database
from .database import close_pool, create_db_pool, get_db
from .passwords import hash_password, verify_password
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

# ── Pydantic schemas ─────────────────────────────────────────────────────────

class LoginRequest(BaseModel):
    user: str
    password: str


class TokenResponse(BaseModel):
    access_token: str


# ── Lifespan ──────────────────────────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Open DB/Redis pools and ensure the users table exists on startup."""
    await create_db_pool()
    await create_redis_pool()

    # Auto-create the users table if it doesn't exist yet.
    async with database.pool.acquire() as conn:
        await conn.execute("""
            CREATE TABLE IF NOT EXISTS users (
                id            SERIAL       PRIMARY KEY,
                username      VARCHAR(50)  UNIQUE NOT NULL,
                password_hash VARCHAR(255) NOT NULL,
                created_at    TIMESTAMPTZ  DEFAULT NOW()
            )
        """)

    yield

    await close_pool()
    await close_redis_pool()


app = FastAPI(title="Auth Service", version="0.1.0", lifespan=lifespan)


# ── Health ────────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    return {"status": "ok"}


# ── POST /auth/login ──────────────────────────────────────────────────────────

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
    # 1. Look up the user by username.
    row = await conn.fetchrow(
        "SELECT id, password_hash FROM users WHERE username = $1",
        body.user,
    )

    if row is None:
        # ── Sign Up ───────────────────────────────────────────────────────
        hashed = hash_password(body.password)
        user_id = await conn.fetchval(
            "INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id",
            body.user,
            hashed,
        )
    else:
        # ── Login ─────────────────────────────────────────────────────────
        if not verify_password(body.password, row["password_hash"]):
            raise HTTPException(status_code=401, detail="Invalid credentials")
        user_id = row["id"]

    # 2. Generate tokens.
    access_token = create_access_token(user_id)
    refresh_token = generate_refresh_token()

    # 3. Store refresh token in Redis.
    await store_refresh_token(refresh_token, user_id)

    # 4. Set refresh token as HttpOnly cookie.
    response.set_cookie(
        key="refresh_token",
        value=refresh_token,
        httponly=True,
        secure=False,       # Set to True when using HTTPS in production
        samesite="lax",
        max_age=REFRESH_TOKEN_TTL_SECONDS,
        path="/",
    )

    return TokenResponse(access_token=access_token)


# ── POST /auth/refresh ───────────────────────────────────────────────────────

@app.post("/auth/refresh", response_model=TokenResponse)
async def refresh(
    refresh_token: str | None = Cookie(default=None),
    conn: asyncpg.Connection = Depends(get_db),
):
    """Exchange a valid refresh token (from cookie) for a new access token."""
    if refresh_token is None:
        raise HTTPException(status_code=401, detail="Missing refresh token")

    # 1. Look up user_id from Redis.
    user_id = await get_user_id_by_token(refresh_token)
    if user_id is None:
        raise HTTPException(status_code=401, detail="Invalid or expired refresh token")

    # 2. Verify user still exists and is active.
    exists = await conn.fetchval(
        "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)",
        user_id,
    )
    if not exists:
        raise HTTPException(status_code=401, detail="User no longer exists")

    # 3. Issue a fresh access token.
    access_token = create_access_token(user_id)
    return TokenResponse(access_token=access_token)




# ── POST /auth/logout ────────────────────────────────────────────────────────

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
