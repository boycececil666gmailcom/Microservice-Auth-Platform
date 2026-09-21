# Endpoint Authentication & Routing Map

This document outlines the architecture, routing rules, and authentication specifications for all endpoints exposed by Amazon API Gateway to downstream AWS Lambda microservices.

## 1. Routing & Architecture

Amazon API Gateway (HTTP API v2) acts as the unified entrypoint for clients, proxying requests directly to backend AWS Lambda functions via `AWS_PROXY` integrations. Asynchronous analytics events are dispatched via Amazon SQS.

```text
+----------------------------------------------------------------------------+
| AWS Serverless Architecture & Routing Map                                  |
+----------------------------------------------------------------------------+
| Client Request (HTTPS)                                                     |
|                            |                                               |
|                            v                                               |
|          +----------------------------------+                              |
|          | Amazon API Gateway (HTTP API v2) |                              |
|          +-----------------+----------------+                              |
|                            |                                               |
|            +---------------+---------------+                               |
|            |                               |                               |
|            v (AWS_PROXY)                   v (AWS_PROXY)                   |
| +----------------------+        +-----------------------+                  |
| | Shortener Lambda     |        | Analytics Lambda      |                  |
| | (provided.al2023)    |        | (provided.al2023)     |                  |
| +----------+-----------+        +-----------+-----------+                  |
|            |                                ^                              |
|            | SQS SendMessage                | SQS Trigger                  |
|            +-----------> [ SQS Queue ] -----+                              |
+----------------------------------------------------------------------------+
```

## 2. Endpoint Specifications

| Method | Endpoint Path | Auth Type | Target Upstream Service | Description |
| :--- | :--- | :--- | :--- | :--- |
| **GET** | `/health` | None | `url-shortener-shortener-dev` | Liveness check verifying shortener service and database connectivity. |
| **GET** | `/ready` | None | `url-shortener-shortener-dev` | Readiness check verifying external dependency status. |
| **POST** | `/api/v1/shorten` | Public (Dev) / IAM / JWT | `url-shortener-shortener-dev` | Creates a new shortened URL entry in PostgreSQL and warms Redis. |
| **GET** | `/api/v1/urls/{shortURL}` | Public (Dev) / IAM / JWT | `url-shortener-shortener-dev` | Retrieves original target URL metadata (reads Redis cache-aside or PostgreSQL). |
| **GET** | `/r/{shortURL}` | None | `url-shortener-shortener-dev` | Public 302 redirect endpoint; dispatches asynchronous tracking event to SQS. |
| **GET** | `/stats` | None | `url-shortener-analytics-dev` | Root path statistics query returning total redirects and counts per short URL. |
| **GET** | `/api/v1/analytics/stats` | Public (Dev) / IAM / JWT | `url-shortener-analytics-dev` | Versioned analytics statistics endpoint querying ElastiCache Redis counters. |

## 3. Asynchronous Event Pipeline

| Component | Technology | Payload Schema | Action |
| :--- | :--- | :--- | :--- |
| **Publisher** | Shortener Lambda | `{"short_url": 12345, "event": "redirect"}` | Dispatched via AWS SDK v2 SQS `SendMessage` |
| **Queue** | Amazon SQS (`url-shortener-redirects-dev`) | Standard JSON | Message retention with Dead-Letter Queue (DLQ) |
| **Consumer** | Analytics Lambda | SQS Event Batch | Ingested via Lambda Event Source Mapping (`batch_size = 10`) |
| **Store** | ElastiCache Redis 7 | Redis Hash / Key | `analytics:total_redirects` and `analytics:redirects_by_short_url` |
