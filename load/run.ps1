param([int]$Rate = 20, [int]$Repeats = 2, [string]$Duration = '60s')
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$results = Join-Path $PSScriptRoot 'results'
New-Item -ItemType Directory -Force -Path $results | Out-Null
docker pull grafana/k6:latest
if ($LASTEXITCODE -ne 0) { throw 'Could not download k6' }
$image = docker image inspect grafana/k6:latest --format '{{.Id}}'
$image | Set-Content (Join-Path $results 'k6-image.txt')
$body = @{ content = ('x' * 1024); visibility = 'public'; expires_in = 3600 } | ConvertTo-Json -Compress
$paste = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/v1/pastes' -ContentType 'application/json' -Headers @{ 'Idempotency-Key' = [guid]::NewGuid().ToString() } -Body $body
# Both protocols read the exact same paste. Warm Redis before each run.
for ($repeat = 1; $repeat -le $Repeats; $repeat++) {
    $protocols = if ($repeat % 2 -eq 1) { @('rest', 'grpc') } else { @('grpc', 'rest') }
    foreach ($operation in @('get', 'create')) {
        foreach ($protocol in $protocols) {
            Invoke-RestMethod -Uri "http://localhost:8080/api/v1/pastes/$($paste.short_code)" | Out-Null
            $run = "$protocol-$operation-$repeat"
            Write-Host "Running $run"
            docker run --rm -v "${repo}:/work:ro" -v "${results}:/results" $image run -e "PROTOCOL=$protocol" -e "OPERATION=$operation" -e "CODE=$($paste.short_code)" -e "RATE=$Rate" -e "DURATION=$Duration" -e "RUN=$run" /work/load/bench.js
            if ($LASTEXITCODE -ne 0) { throw "Test $run failed; inspect the output and results." }
        }
    }
}
$rows = Get-ChildItem $results -Filter '*-compact.json' | ForEach-Object {
    $r = Get-Content $_.FullName -Raw | ConvertFrom-Json
    [pscustomobject]@{ Run = $_.BaseName; p50_ms = $r.latency_ms.med; p95_ms = $r.latency_ms.'p(95)'; p99_ms = $r.latency_ms.'p(99)'; ErrorRate = $r.failures.rate; SuccessPerSec = $r.successful.rate; Dropped = $r.dropped.count }
}
$rows | Export-Csv (Join-Path $results 'comparison.csv') -NoTypeInformation
$rows | Format-Table -AutoSize
