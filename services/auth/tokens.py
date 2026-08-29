import secrets
from datetime import UTC, datetime, timedelta

import jwt
import redis.asyncio as aioredis

from .config import (
    JWT_ALGORITHM,
    JWT_EXPIRATION_MINUTES,
    JWT_PRIVATE_KEY,
    REDIS_URL,
    REFRESH_TOKEN_TTL_SECONDS,
)

# region Redis Connection Pool
redis_pool: aioredis.Redis | None = None


async def create_redis_pool() -> None:
    """Open the Redis connection pool. Call this once at app startup."""
    global redis_pool
    redis_pool = aioredis.from_url(REDIS_URL, decode_responses=True)


async def close_redis_pool() -> None:
    """Close the Redis connection pool. Call this at app shutdown."""
    if redis_pool:
        await redis_pool.aclose()


# endregion


# region Access Token & Refresh Token Management
def create_access_token(email: str, sso_provider: str = "local") -> str:
    """Create a signed RS256 JWT access token with single universal OIDC schema."""
    payload = {
        "sub": email,
        "email": email,
        "sso_provider": sso_provider,
        "exp": datetime.now(UTC) + timedelta(minutes=JWT_EXPIRATION_MINUTES),
        "iat": datetime.now(UTC),
    }
    return jwt.encode(payload, JWT_PRIVATE_KEY, algorithm=JWT_ALGORITHM)


def generate_refresh_token() -> str:
    """Generate a cryptographically secure opaque refresh token."""
    return secrets.token_urlsafe(48)


def _token_key(token: str) -> str:
    """Canonical Redis key for a given refresh token."""
    return f"refresh_token:{token}"


async def store_refresh_token(token: str, email: str) -> None:
    """Store a refresh token in Redis mapped to user email with a 30-day TTL."""
    await redis_pool.set(
        _token_key(token),
        email,
        ex=REFRESH_TOKEN_TTL_SECONDS,
    )


async def get_email_by_token(token: str) -> str | None:
    """Look up the email associated with a refresh token.

    Returns None if the token is expired or does not exist.
    """
    return await redis_pool.get(_token_key(token))


async def delete_refresh_token(token: str) -> None:
    """Revoke a refresh token by removing it from Redis."""
    await redis_pool.delete(_token_key(token))


# endregion
