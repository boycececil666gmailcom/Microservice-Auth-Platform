import os
import secrets
from datetime import datetime, timedelta, timezone

import jwt
import redis.asyncio as aioredis

# ── RS256 Private Key (used to SIGN tokens) ──────────────────────────────────
JWT_ALGORITHM = "RS256"
JWT_EXPIRATION_MINUTES = int(os.environ.get("JWT_EXPIRATION_MINUTES", "15"))
JWT_PRIVATE_KEY = os.environ.get("JWT_PRIVATE_KEY")

if not JWT_PRIVATE_KEY:
    raise RuntimeError("Auth service requires JWT_PRIVATE_KEY env var")

REDIS_URL = os.environ["REDIS_URL"]

# Refresh tokens expire after 30 days.
REFRESH_TOKEN_TTL_SECONDS = int(os.environ.get("REFRESH_TOKEN_TTL_SECONDS", str(60 * 60 * 24 * 30)))

# Module-level pool, created once at startup.
redis_pool: aioredis.Redis | None = None


async def create_redis_pool() -> None:
    """Open the Redis connection pool. Call this once at app startup."""
    global redis_pool
    redis_pool = aioredis.from_url(REDIS_URL, decode_responses=True)


async def close_redis_pool() -> None:
    """Close the Redis connection pool. Call this at app shutdown."""
    if redis_pool:
        await redis_pool.aclose()


def create_access_token(user_id: int) -> str:
    """Create a signed JWT access token with a 15-minute expiration."""
    payload = {
        "sub": str(user_id),
        "exp": datetime.now(timezone.utc) + timedelta(minutes=JWT_EXPIRATION_MINUTES),
        "iat": datetime.now(timezone.utc),
    }
    return jwt.encode(payload, JWT_PRIVATE_KEY, algorithm=JWT_ALGORITHM)


def generate_refresh_token() -> str:
    """Generate a cryptographically secure opaque refresh token."""
    return secrets.token_urlsafe(48)


def _token_key(token: str) -> str:
    """Canonical Redis key for a given refresh token."""
    return f"refresh_token:{token}"


async def store_refresh_token(token: str, user_id: int) -> None:
    """Store a refresh token in Redis with a 30-day TTL."""
    await redis_pool.set(
        _token_key(token),
        str(user_id),
        ex=REFRESH_TOKEN_TTL_SECONDS,
    )


async def get_user_id_by_token(token: str) -> int | None:
    """Look up the user_id associated with a refresh token.

    Returns None if the token is expired or does not exist.
    """
    user_id = await redis_pool.get(_token_key(token))
    if user_id is None:
        return None
    return int(user_id)


async def delete_refresh_token(token: str) -> None:
    """Revoke a refresh token by removing it from Redis."""
    await redis_pool.delete(_token_key(token))
