# Storage Design

This document describes the primary datastore (**Amazon RDS PostgreSQL**) and caching layer (**Amazon ElastiCache Redis**) supporting the URL Shortener service.

## 1. Primary Database (Amazon RDS PostgreSQL)

- **Engine**: PostgreSQL 16.9
- **Instance Class**: `db.t4g.micro` (ARM64 Graviton2, 2 vCPUs, 1 GiB RAM)
- **Storage**: 20 GiB gp3 (fixed cap with `max_allocated_storage = 20`)
- **Backup Policy**: `backup_retention_period = 0` (cost-minimized for dev/test)

### Schema Definition
```sql
CREATE TABLE IF NOT EXISTS urls (
    short_url  BIGSERIAL PRIMARY KEY,
    long_url   TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Implicit unique index on long_url enforces deduplication and enables fast lookups:
-- CREATE UNIQUE INDEX urls_long_url_key ON urls(long_url);
```

---

## 2. In-Memory Cache (Amazon ElastiCache Redis)

- **Engine**: Redis 7 (standalone)
- **Node Type**: `cache.t4g.micro` (1 node)
- **Port**: 6379 (VPC Security Group protected)
- **Snapshot Policy**: `snapshot_retention_limit = 0`

### Key Schema

| Key Pattern | Data Type | Value (JSON) | TTL |
| :--- | :--- | :--- | :--- |
| `url:{short_url}` | String (JSON) | `{"short_url": 1, "long_url": "https://...", "created_at": "..."}` | 24 Hours (Configurable) |

---

## 3. Cache-Aside Operation Pattern

For read queries (`GET /api/v1/urls/{shortURL}` and `GET /r/{shortURL}`):

```text
               +----------------------+
               | Read Request Arrives |
               +-----------+----------+
                           |
                           v
               +----------------------+
               | Check Redis Cache    |
               +-----------+----------+
                           |
            +--------------+--------------+
            |                             |
      (Cache Hit)                   (Cache Miss)
            v                             v
+-----------------------+     +-----------------------+
| Return cached URL     |     | Query RDS PostgreSQL  |
| record immediately    |     +-----------+-----------+
+-----------------------+                 |
                                          v
                              +-----------------------+
                              | Set Redis cache (TTL) |
                              | & return URL record   |
                              +-----------------------+
```

1. Shortener Lambda checks `url:{short_url}` in ElastiCache Redis.
2. **Cache Hit**: Returns the parsed URL record immediately (< 1 ms latency).
3. **Cache Miss / Error**: Falls back to querying RDS PostgreSQL, asynchronously writes the record back to Redis with a 24-hour TTL, and returns the result.
