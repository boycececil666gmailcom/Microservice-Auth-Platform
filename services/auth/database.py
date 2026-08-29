import asyncpg

from .config import DATABASE_URL

# A connection pool is created once at startup and shared across all requests.
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


# region Database Migration & Schema Setup
async def init_users_table() -> None:
    """Ensure the users table with email PRIMARY KEY exists in PostgreSQL."""
    async with pool.acquire() as conn:
        await conn.execute("""
            CREATE TABLE IF NOT EXISTS users (
                email         VARCHAR(255) PRIMARY KEY,
                password_hash VARCHAR(255) NOT NULL,
                sso_provider  VARCHAR(50)  DEFAULT 'local',
                google_sub    VARCHAR(255),
                created_at    TIMESTAMPTZ  DEFAULT NOW()
            );
        """)


# endregion
