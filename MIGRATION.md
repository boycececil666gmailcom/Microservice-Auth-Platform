# PostgreSQL/Redis to DynamoDB migration

The new Terraform configuration removes the legacy VPC, RDS, and ElastiCache resources. Do not apply it over an existing environment until its data has been exported and the plan has been reviewed.

## Safe rollout

1. Back up PostgreSQL and create a final RDS snapshot.
2. Export `short_url`, `long_url`, and `created_at` from the `urls` table.
3. Deploy the new stack under a temporary environment name so the legacy stack remains untouched.
4. Recreate each URL through `POST /api/v1/shorten`. IDs intentionally change from predictable integers to URL-derived strings; update any externally distributed links or retain the old redirect service during a transition window.
5. Run the end-to-end suite against the temporary endpoint.
6. Move DNS or clients to the new endpoint.
7. Retain the RDS snapshot and old Terraform state for the agreed recovery window before removing legacy resources.

Historical aggregate analytics can be archived, but should not be written directly into the new table without a reviewed import tool. The new analytics schema contains idempotency records as well as counters.

## Terraform state

The repository now declares an S3 backend with native state locking. Initialize each environment with its own `backend.hcl` key. When moving an existing local state, first copy it to a safe location, then use Terraform's prompted backend migration flow. Never discard the only state copy.
