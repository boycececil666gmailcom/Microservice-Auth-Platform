"""
Auth Service — handles user registration, password login, token refresh, logout, and Google OIDC.

Endpoints:
  POST /auth/login           — Sign up (if user doesn't exist) or login with password.
  POST /auth/refresh         — Exchange a refresh token for a new access token.
  POST /auth/logout          — Revoke the refresh token.
  POST /auth/google/callback — Exchange Google ID token / code for RS256 JWT & refresh cookie.
"""

from contextlib import asynccontextmanager

import asyncpg
from fastapi import Cookie, Depends, FastAPI, HTTPException, Response

from . import oidc
from .database import close_pool, create_db_pool, get_db, init_users_table
from .passwords import hash_password, verify_password
from .schemas import GoogleCallbackRequest, LoginRequest, TokenResponse
from .tokens import (
    REFRESH_TOKEN_TTL_SECONDS,
    close_redis_pool,
    create_access_token,
    create_redis_pool,
    delete_refresh_token,
    generate_refresh_token,
    get_email_by_token,
    store_refresh_token,
)


# region Lifespan & App Setup
@asynccontextmanager
async def lifespan(app: FastAPI):
    """Open DB/Redis pools and ensure the users table and Google OIDC columns exist."""
    await create_db_pool()
    await create_redis_pool()

    # Auto-create the users table with email PRIMARY KEY if not exists yet.
    await init_users_table()

    yield

    await close_pool()
    await close_redis_pool()


# endregion


app = FastAPI(title="Auth Service", version="0.1.0", lifespan=lifespan)


# region Health Endpoints
@app.get("/health")
async def health():
    return {"status": "ok"}


# endregion


# region Password Authentication Routes
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
        "SELECT password_hash, sso_provider FROM users WHERE email = $1",
        body.email,
    )

    if row is None:
        # Sign Up
        hashed = hash_password(body.password)
        await conn.execute(
            "INSERT INTO users (email, password_hash, sso_provider) VALUES ($1, $2, 'local')",
            body.email,
            hashed,
        )
        sso_provider = "local"
    else:
        # Login
        if not verify_password(body.password, row["password_hash"]):
            raise HTTPException(status_code=401, detail="Invalid credentials")
        sso_provider = row["sso_provider"] or "local"

    access_token = create_access_token(email=body.email, sso_provider=sso_provider)
    refresh_token = generate_refresh_token()

    await store_refresh_token(refresh_token, body.email)

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

    email = await get_email_by_token(refresh_token)
    if email is None:
        raise HTTPException(status_code=401, detail="Invalid or expired refresh token")

    user_row = await conn.fetchrow(
        "SELECT sso_provider FROM users WHERE email = $1",
        email,
    )
    if user_row is None:
        raise HTTPException(status_code=401, detail="User no longer exists")

    access_token = create_access_token(
        email=email,
        sso_provider=user_row["sso_provider"] or "local",
    )
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


# endregion


# region Google OpenID Connect (OIDC) Endpoints


@app.post("/auth/google/callback", response_model=TokenResponse)
async def google_callback(
    body: GoogleCallbackRequest,
    response: Response,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Google OIDC Callback — exchanges authorization code with Google for ID token,
    auto-provisions Google user, and issues signed RS256 JWT access token and refresh cookie.
    """
    try:
        id_token = await oidc.exchange_code_for_id_token(
            code=body.code,
            redirect_uri=body.redirect_uri or oidc.GOOGLE_REDIRECT_URI,
        )
    except ValueError as err:
        raise HTTPException(status_code=400, detail=f"Google Code Exchange failed: {err}")

    try:
        user_info = oidc.parse_google_id_token(id_token)
    except ValueError as err:
        raise HTTPException(status_code=400, detail=f"Invalid Google ID Token: {err}")

    email = user_info["email"]
    google_sub = user_info["google_sub"]

    # 1. Resolve or auto-provision user by Google Sub or Email.
    row = await conn.fetchrow(
        "SELECT email FROM users WHERE google_sub = $1 OR email = $2",
        google_sub,
        email,
    )

    if row is None:
        await conn.execute(
            """INSERT INTO users (email, password_hash, sso_provider, google_sub)
               VALUES ($1, 'GOOGLE_OIDC_USER_NO_PASSWORD', 'google_oidc', $2)""",
            email,
            google_sub,
        )

    # 2. Generate identical RS256 JWT Access Token and Refresh Token Cookie.
    access_token = create_access_token(email=email, sso_provider="google_oidc")
    refresh_token = generate_refresh_token()

    await store_refresh_token(refresh_token, email)

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


# endregion
