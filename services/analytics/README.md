# Analytics Service

This service consumes validated redirect events from Kafka and exposes concurrency-safe in-memory counters. Counters are intentionally ephemeral in this reference deployment; production analytics should write events or aggregates to durable storage before committing Kafka offsets.
