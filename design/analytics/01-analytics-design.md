# Analytics design

Redirects enqueue `{"short_url":"<id>","event":"redirect"}` to SQS. The analytics Lambda consumes batches and returns failed message identifiers so Lambda retries only failed records.

For each message, a DynamoDB transaction performs three writes:

1. Create `EVENT#{SQS message ID}` with a 15-day TTL and a not-exists condition.
2. Increment the `TOTAL` counter.
3. Increment the `URL#{short URL}` counter.

If the event record already exists, the retry is treated as successful without incrementing again. The transaction prevents partial counter updates. Repeated failures move to the encrypted dead-letter queue, whose visible-message metric has a CloudWatch alarm.

The stats endpoint scans only `TOTAL` and `URL#` metric items. This is deliberately simple; replace it with pagination or precomputed top-N views before the number of shortened URLs becomes large.
