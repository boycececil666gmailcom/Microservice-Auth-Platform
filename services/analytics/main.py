import asyncio
import json
import logging
import os
from contextlib import asynccontextmanager

from aiokafka import AIOKafkaConsumer
from fastapi import FastAPI

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.INFO)

KAFKA_BROKER_URL = os.environ.get("KAFKA_BROKER_URL", "kafka:9092")
KAFKA_TOPIC = "url-redirects"

# In-memory counter for analytics
analytics_data = {"total_redirects": 0, "redirects_by_short_url": {}}

consumer_task = None


async def consume_events():
    """Background task to consume events from Kafka."""
    consumer = AIOKafkaConsumer(
        KAFKA_TOPIC,
        bootstrap_servers=KAFKA_BROKER_URL,
        group_id="analytics-group",
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        auto_offset_reset="earliest",
    )

    # Wait for Kafka to be ready
    for i in range(10):
        try:
            await consumer.start()
            logger.info("Successfully connected to Kafka")
            break
        except Exception as e:
            logger.warning(f"Waiting for Kafka... {e}")
            await asyncio.sleep(2)
    else:
        logger.error("Failed to connect to Kafka")
        return

    try:
        async for msg in consumer:
            event = msg.value
            logger.info(f"Received event: {event}")

            # Update analytics data
            analytics_data["total_redirects"] += 1
            short_url = event.get("short_url")
            if short_url:
                analytics_data["redirects_by_short_url"][short_url] = (
                    analytics_data["redirects_by_short_url"].get(short_url, 0) + 1
                )

    finally:
        await consumer.stop()


@asynccontextmanager
async def lifespan(app: FastAPI):
    global consumer_task
    # Start the Kafka consumer task
    consumer_task = asyncio.create_task(consume_events())
    yield
    # Cancel consumer on shutdown
    if consumer_task:
        consumer_task.cancel()
        try:
            await consumer_task
        except asyncio.CancelledError:
            pass


app = FastAPI(title="Analytics Service", version="0.1.0", lifespan=lifespan)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/stats")
async def get_stats():
    return analytics_data
