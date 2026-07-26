# S3-Compatible Object Storage

Wreckr generates detailed performance reports containing individual latency statistics, threshold evaluations, and complete HTTP response records for every single simulated request. For high-scale or long-running tests, this raw JSON report can grow to tens or hundreds of megabytes.

To prevent bloating PostgreSQL and exhausting memory, Wreckr uses a **hybrid storage architecture** that offloads large, raw JSON payloads to S3-compatible Object Storage.

## Architecture

1. **PostgreSQL / Memory Store:** Stores lightweight metadata, including run status, scenario versions, run durations, target metadata, and sequential event logs. The actual detailed JSON report column (`raw_report`) is dropped from the database (migrated in `000002_drop_raw_report.up.sql`).
2. **Blob Store:** Stores the complete, massive raw JSON report under the key `runs/{run_id}/report.json`.

```
                    ┌────────────────────────┐
                    │       HTTP API         │
                    └───────────┬────────────┘
                                │
                  Get Report    │    Store Metadata
               (Checks Blob)    │    (Status, Events)
                                ▼
  ┌────────────────┐        ┌───┴────────────┐
  │   Object Store │◄───────┤   PostgreSQL   │
  │ (S3 or Memory) │        └────────────────┘
  └────────────────┘
```

When client requests the detailed report via `GET /v1/runs/{id}/report`:
- The API first checks if a `BlobStore` is configured.
- If configured, it directly fetches `runs/{id}/report.json` from the object store and streams it to the client.
- If the object store is unavailable or not configured, it falls back to the local memory store or database `record.Report` snapshot (for local fallback development).

## Configuration

Configure Object Storage with the following environment variables. If these variables are not provided, Wreckr defaults to using an in-memory/in-database fallback storage.

| Environment Variable | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `WRECKR_S3_ENDPOINT` | String | `""` | Endpoint URL (e.g., `http://localhost:9000` for MinIO, or empty for AWS S3). |
| `WRECKR_S3_REGION` | String | `"us-east-1"` | AWS S3 region. |
| `WRECKR_S3_BUCKET` | String | `"wreckr-artifacts"` | S3 bucket name. |
| `WRECKR_S3_ACCESS_KEY_ID` | String | `""` | Access Key ID. |
| `WRECKR_S3_SECRET_ACCESS_KEY` | String | `""` | Secret Access Key. |
| `WRECKR_S3_DISABLE_SSL` | Boolean | `false` | Set to true to disable SSL (useful for local MinIO over HTTP). |
| `WRECKR_S3_FORCE_PATH_STYLE` | Boolean | `false` | Forces path-style addressing (required by MinIO). |

## Local Development with MinIO / LocalStack

You can easily spin up a local S3-compatible store using MinIO in Docker Compose:

```yaml
services:
  minio:
    image: minio/minio
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: wreckr
      MINIO_ROOT_PASSWORD: wreckrpassword
    command: server /data --console-address ":9001"
```

Configure Wreckr to use it:

```bash
WRECKR_S3_ENDPOINT=http://localhost:9000
WRECKR_S3_REGION=us-east-1
WRECKR_S3_BUCKET=wreckr-artifacts
WRECKR_S3_ACCESS_KEY_ID=wreckr
WRECKR_S3_SECRET_ACCESS_KEY=wreckrpassword
WRECKR_S3_DISABLE_SSL=true
WRECKR_S3_FORCE_PATH_STYLE=true
```
