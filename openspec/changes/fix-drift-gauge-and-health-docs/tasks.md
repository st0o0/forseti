## 1. Fix stale drift gauge

- [x] 1.1 Change `buildDriftMap` in `internal/worker/worker.go` to always emit all 7 resource type keys (groups, adlists, deny, allow, local_dns, cname, clients) with count 0 when no drift exists
- [x] 1.2 Add test in `internal/worker/worker_test.go`: verify that after a cycle with adlist drift followed by a cycle without drift, the recorded drift map contains `adlists: 0`
- [x] 1.3 Update existing `internal/metrics/metrics_test.go` drift tests if any assume sparse map behavior

## 2. Document health metric encoding

- [x] 2.1 Add encoding note to `forseti_target_health` row in README.md metrics table: `0 = healthy, 1 = degraded, 2 = down`

## 3. Verify

- [x] 3.1 Run `go test -race ./...` and `golangci-lint run`
