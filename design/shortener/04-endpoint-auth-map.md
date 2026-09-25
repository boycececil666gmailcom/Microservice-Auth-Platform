# Endpoint and access map

| Method | Path | Access | Lambda |
| --- | --- | --- | --- |
| GET | `/health` | Public | Shortener |
| GET | `/ready` | Public | Shortener |
| POST | `/api/v1/shorten` | Public, throttled | Shortener |
| GET | `/api/v1/urls/{shortURL}` | Public, throttled | Shortener |
| GET | `/r/{shortURL}` | Public, throttled | Shortener |
| GET | `/api/v1/analytics/stats` | Public, throttled | Analytics |

API Gateway applies a default token-bucket throttle. Public access is suitable for a demonstration service. A production product with user-specific links or private analytics must add an authorizer and ownership checks; throttling alone is not authentication.
