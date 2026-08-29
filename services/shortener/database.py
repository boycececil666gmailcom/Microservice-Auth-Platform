import os

import asyncpg

# Read the connection string injected by k8s ConfigMap (or a local .env).
# Example: postgresql://postgres:postgres@db:5432/urlshortener
DATABASE_URL = os.environ["DATABASE_URL"]

# A connection pool is created once at startup and shared across all requests.
# asyncpg manages a set of open TCP connections to Postgres so each request
# doesn't pay the cost of establishing a new connection every time.
pool: asyncpg.Pool | None = None


async def create_db_pool() -> None:
    """Open the connection pool. Call this once at app startup."""
    global pool
    pool = await asyncpg.create_pool(DATABASE_URL)


async def close_pool() -> None:
    """Close all connections in the pool. Call this at app shutdown."""
    if pool:
        await pool.close()


async def get_db() -> asyncpg.Connection:
    """FastAPI dependency — borrows one connection from the pool per request."""
    async with pool.acquire() as conn:
        yield conn
