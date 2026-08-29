import asyncio
import json
import os
from contextlib import asynccontextmanager

import asyncpg
from aiokafka import AIOKafkaProducer
from fastapi import Depends, FastAPI, HTTPException
from fastapi.responses import RedirectResponse

from .cache import close_redis_pool, create_redis_pool, get_cached_url, set_cached_url
from .crud import create_new_unique_long_url, get_url_by_id
from .database import close_pool, create_db_pool, get_db
from .schemas import ShortenRequest, URLCreateResponse, URLLookupResponse

# region Configuration & Lifespan
KAFKA_BROKER_URL = os.environ.get("KAFKA_BROKER_URL", "kafka:9092")
KAFKA_TOPIC = "url-redirects"
kafka_producer = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global kafka_producer
    """Open the DB pool and create the table on startup; close pool on shutdown."""
    await create_db_pool()
    await create_redis_pool()

    # Create the urls table if it doesn't exist yet.
    from .database import pool

    async with pool.acquire() as conn:
        await conn.execute("""
            CREATE TABLE IF NOT EXISTS urls (
                short_url  BIGSERIAL PRIMARY KEY,
                long_url   TEXT        NOT NULL UNIQUE,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        """)

    # Start Kafka Producer
    try:
        kafka_producer = AIOKafkaProducer(
            bootstrap_servers=KAFKA_BROKER_URL,
            value_serializer=lambda v: json.dumps(v).encode("utf-8"),
        )
        await kafka_producer.start()
    except Exception as e:
        print(f"Failed to start Kafka producer: {e}")

    yield

    if kafka_producer:
        await kafka_producer.stop()
    await close_pool()
    await close_redis_pool()


app = FastAPI(title="Shortener Service", version="0.1.0", lifespan=lifespan)
# endregion


# region Health Endpoints
@app.get("/health")
async def health():
    return {"status": "ok"}


# endregion


# region Write Path Endpoints
@app.post("/shorten", response_model=URLCreateResponse, status_code=201)
async def shorten_url(
    body: ShortenRequest,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Generate a short URL from a given long URL.

    Idempotent: submitting the same long_url twice returns the same short_url.
    """
    url_row = await create_new_unique_long_url(conn, str(body.long_url))
    return URLCreateResponse(
        short_url=url_row["short_url"],
        long_url=url_row["long_url"],
        created_at=url_row["created_at"],
    )


# endregion


# region Read Path Endpoints
@app.get("/urls/{short_url}", response_model=URLLookupResponse)
async def get_url(
    short_url: int,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Look up a short URL and return the original long URL.

    Cache-aside: check Redis first; on a miss, query Postgres and warm the cache.
    """
    # Cache hit
    cached = await get_cached_url(short_url)
    if cached is not None:
        return URLLookupResponse(**cached)

    # Cache miss: fall back to Postgres
    url_row = await get_url_by_id(conn, short_url)
    if url_row is None:
        raise HTTPException(status_code=404, detail="Short URL not found")

    # Warm the cache so the next request is served from Redis.
    await set_cached_url(url_row)

    return URLLookupResponse(
        short_url=url_row["short_url"],
        long_url=url_row["long_url"],
        created_at=url_row["created_at"],
    )


async def send_analytics_event(short_url: int):
    if kafka_producer:
        try:
            event = {"short_url": short_url, "event": "redirect"}
            await kafka_producer.send_and_wait(KAFKA_TOPIC, event)
        except Exception as e:
            print(f"Failed to send Kafka event: {e}")


@app.get("/r/{short_url}")
async def redirect(
    short_url: int,
    conn: asyncpg.Connection = Depends(get_db),
):
    """Redirect to the original long URL (HTTP 302).

    Cache-aside: same strategy as get_url — Redis first, Postgres on miss.
    """
    # Cache hit
    cached = await get_cached_url(short_url)
    if cached is not None:
        asyncio.create_task(send_analytics_event(short_url))
        return RedirectResponse(url=cached["long_url"], status_code=302)

    # Cache miss: fall back to Postgres
    url_row = await get_url_by_id(conn, short_url)
    if url_row is None:
        raise HTTPException(status_code=404, detail="Short URL not found")

    await set_cached_url(url_row)
    asyncio.create_task(send_analytics_event(short_url))
    return RedirectResponse(url=url_row["long_url"], status_code=302)


# endregion
