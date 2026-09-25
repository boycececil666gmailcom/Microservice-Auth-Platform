# URL storage design

The shortener uses one on-demand DynamoDB table keyed by `short_url`.

| Attribute | Type | Purpose |
| --- | --- | --- |
| `short_url` | String, partition key | URL-safe lookup identifier |
| `long_url` | String | Validated HTTP(S) destination |
| `created_at` | String | RFC3339 creation timestamp |

The first identifier is the first 16 base64url characters of SHA-256 over the destination URL (96 bits). It is stable for repeat requests and does not expose creation volume. A conditional put prevents overwrites. If an actual hash-prefix collision is found, the service retries with 22, 32, and finally all 43 encoded characters.

Reads are strongly consistent so a newly created URL can be resolved immediately. DynamoDB removes the cache-invalidation and database-connection concerns that existed in the former PostgreSQL plus Redis design. Production enables point-in-time recovery and deletion protection.
