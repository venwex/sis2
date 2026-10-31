# Pastebin — SIS #2

Go service with a REST/JSON baseline and gRPC/Protobuf interface. Both share the service layer, Redis cache and PostgreSQL repository. Anonymous public pastes only.

## Architecture

```mermaid
flowchart LR
    C[Go client / k6] -->|HTTP JSON| H[REST handler :8080]
    C -->|gRPC Protobuf| G[gRPC handler :9090]
    H --> S[Service]
    G --> S
    S -->|cache-aside reads| R[(Redis)]
    S --> P[Repository / pgxpool]
    P --> D[(PostgreSQL)]
    M[Prometheus :9091] -->|scrape /metrics| H
    F[Grafana :3000] -->|query| M
```

Create writes to PostgreSQL. Get checks Redis, falls back to PostgreSQL on cache miss/error, then caches the result. Cache TTL is bounded by paste expiration. Random eight-character Base62 codes are protected by a UNIQUE constraint.

REST is the readable public API baseline. gRPC provides a typed contract and compact encoding for service clients. Performance is measured rather than assumed. Contract: `proto/paste.proto`; generated code: `gen`.

## Run

Requirements: Docker Desktop, Go matching go.mod, Goose for migrations. PowerShell commands from the repository root:

```powershell
docker compose up -d --build
goose -dir ./internal/migrations postgres "postgres://pastebin:pastebin_local@localhost:5432/pastebin?sslmode=disable" up
go run ./cmd/client
```

Compose supplies application environment variables. PostgreSQL is `postgres:5432` inside Docker and `localhost:5432` from the host. Migrations must be applied separately. Credentials are for local coursework.

REST: `POST /api/v1/pastes` accepts content, visibility=public, expires_in seconds and a UUID Idempotency-Key header; `GET /api/v1/pastes/{code}` reads a paste. gRPC methods are paste.PasteService/CreatePaste and GetPaste; Create requires idempotency-key metadata. Local gRPC uses plaintext.

## Reliability

Idempotency keys and request hashes are stored in PostgreSQL. Key reservation, paste creation and saving the original result occur in one transaction. Same key/input returns the original result across transports. Different input is rejected with HTTP 409 / gRPC AlreadyExists. Short-code collision regeneration is separate from network retries.

Handlers impose a 3-second service deadline. Redis operations have a 200 ms budget. PostgreSQL receives the request context. The Go client allows at most 3 attempts, 2 seconds each, within 5 seconds overall. Selected transient failures trigger exponential backoff with full jitter starting at 100 ms. Create retries preserve input and key; validation errors are not retried.

## Monitoring

- Application metrics: http://localhost:8080/metrics
- Prometheus targets: http://localhost:9091/targets (target should be UP)
- Grafana: http://localhost:3000; Prometheus data source URL: `http://prometheus:9090`

Panel queries:

```promql
sum by (protocol, operation) (pastebin_requests_total)
sum by (protocol, operation) (rate(pastebin_requests_total[1m]))
1000 * histogram_quantile(0.95, sum by (le, protocol, operation) (rate(pastebin_request_duration_seconds_bucket[5m])))
```

The last query estimates server p95 in milliseconds. REST middleware and gRPC interceptors cover different serialization boundaries; use client measurements for protocol comparison.

## Benchmark

```powershell
# Smoke check
powershell -ExecutionPolicy Bypass -File .\load\run.ps1 -Rate 2 -Repeats 1 -Duration 5s
# Main comparison: approximately eight minutes
powershell -ExecutionPolicy Bypass -File .\load\run.ps1 -Rate 20 -Repeats 2 -Duration 60s
# Serialized request sizes
go run ./load/payload
```

Main run: four scenarios (REST/gRPC x create/get), two repetitions, alternating protocol order. Each targets 20 requests/second for 60 seconds. Content is 1024 ASCII bytes; expiration is 3600 seconds. Create uses fresh UUID keys. Get reads the same prewarmed paste: this is a hot-key Redis workload, not a database read benchmark. Connections are reused. gRPC connection setup is excluded; initial HTTP setup can be included. Custom client latency has millisecond resolution. No benchmark retries are enabled.

Results: `load/results/comparison.csv`, per-run compact JSON and full summaries; k6 image ID is recorded. Report p50/p95/p99, successful requests/second, error rate and dropped iterations per protocol/operation. CSV updates only when the entire runner completes and may contain older smoke results during execution. Compact JSON records rate/duration. Capture Grafana for the same interval. Record machine specs and Docker resource limits with the submission.

Measured serialized Create request bodies for identical fields:

| Encoding | Bytes |
|---|---:|
| JSON | 1078 |
| Protobuf | 1038 |

Protobuf is 3.71% smaller for this input. This excludes headers, metadata, framing and responses; it is not total wire traffic. Text dominates both payloads.

## Verification and limitations

Confirmed: REST creation, gRPC replay returning the same short code, and reading original content through both interfaces. Four smoke scenarios passed without errors or dropped iterations. Main-run evidence is in the benchmark artifacts; ten-request smoke percentiles are insufficient for conclusions.

Concurrent idempotency races and retries under injected failures have not yet been verified. k6 does not exercise the Go retry client. Registration, automatic idempotency cleanup, replication and failover are outside this implementation. Database tuning belongs to SIS #3. Local measurements are not a production capacity claim.
