# SIS 2 benchmark

From the repository root in PowerShell:

```powershell
# Smoke run: four short tests, stop on any failed check.
powershell -ExecutionPolicy Bypass -File .\load\run.ps1 -Rate 2 -Repeats 1 -Duration 5s
# Measurements: eight tests, about eight minutes. Alternates protocol order.
powershell -ExecutionPolicy Bypass -File .\load\run.ps1 -Rate 20 -Repeats 2 -Duration 60s
go run ./load/payload
```

Docker Desktop and the application must be running; database migrations must be applied. No local k6 installation required. Results are saved under `load/results`, including full summaries, compact summaries, CSV and the k6 image ID. Each run replaces files with the same names.

Workload: 1024 ASCII bytes, public pastes, expiry 3600 seconds. Create uses a new UUID key for each operation. Get reads one shared, prewarmed paste: this is a hot-key/cache benchmark, not a database benchmark. Tests create real disposable pastes but do not delete existing data.

Both protocols use the same arrival rate and duration. Custom latency_ms measures elapsed client call time in milliseconds; gRPC connection setup is excluded, initial HTTP setup can be included. Connections are reused. No retries. Checks validate status and returned data. Failed requests contribute to latency when the call has started; inspect error rate alongside percentiles. Zero dropped iterations and zero errors are required. Date.now has millisecond resolution; do not interpret submillisecond differences. This is a small local comparison, not a production capacity claim. Repeat runs and report both results rather than claiming significance from one number.

The payload program measures the actual JSON and Protobuf serialized Create request bodies for identical fields. It excludes HTTP headers, HTTP/2 framing, gRPC envelope, metadata and response bodies. HTTP GET has no request body; do not compare it to a protobuf body as equivalent wire traffic.

For the submission, include comparison.csv values (p50/p95/p99, success rate, errors), payload output, machine specs and Docker resource limits, the workload above, and Grafana screenshots for the corresponding time range. Monitoring panels show server activity; k6 is the source for client latency. These tests do not validate retry behavior under injected failures or concurrent idempotency races.
