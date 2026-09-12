## 1. Readiness for plan/apply + tests

- [x] 1.1 Add `CheckReadiness()` call in `runPlan()` after client login, skip target on failure
- [x] 1.2 Add `CheckReadiness()` call in `runApply()` after client login, skip target on failure
- [x] 1.3 Write test for `CheckReadiness()` — successful FTL info response
- [x] 1.4 Write test for `CheckReadiness()` — FTL not started (pid=0)
- [x] 1.5 Write test for `CheckReadiness()` — connection refused
- [x] 1.6 Write test for `CheckAlive()` — server responds
- [x] 1.7 Write test for `CheckAlive()` — server unreachable

## 2. Structured error types

- [x] 2.1 Add `ParseAPIErrorKey(error) string` — extract `error.key` from Pi-hole JSON error body
- [x] 2.2 Refactor `IsTransient()` to use error key ("database_error") instead of string matching on message
- [x] 2.3 Refactor `IsGravityCorrupted()` to check for "no such table" in hint field, not message
- [x] 2.4 Update existing tests to verify structured parsing
- [x] 2.5 Add test: parse error key from real Pi-hole error JSON

## 3. Phantom-delete investigation + fix

- [x] 3.1 Add debug logging in `diffAdlists` to log actual vs desired addresses for delete candidates
- [x] 3.2 Add individual delete fallback: if `DeleteAdlists` (batchDelete) returns 404, try individual `DELETE /api/lists/{address}` for each
- [x] 3.3 Add `DeleteAdlist(address string) error` method to pihole.Client using `DELETE /api/lists/{address}`
- [x] 3.4 Write test for individual delete fallback

## 4. Write retry on transient errors

- [x] 4.1 Add `retryWrite` helper in reconciler — retry create/update on transient errors (readonly database, database_error), max 3 attempts, 1s/2s/4s backoff
- [x] 4.2 Wrap `CreateAdlist`, `CreateDomain`, `CreateGroup`, `CreateClient` calls in `retryWrite`
- [x] 4.3 Wrap `UpdateAdlist`, `UpdateClient`, `UpdateDomain` calls in `retryWrite`
- [x] 4.4 Write test: create fails with transient error, succeeds on retry
- [x] 4.5 Write test: create fails with permanent error, no retry

## 5. Async gravity

- [x] 5.1 Add `TriggerAsync(target, reason string)` to gravity.Scheduler — sends to channel, returns immediately
- [x] 5.2 Add gravity worker goroutine in Scheduler.Start that reads from channel and calls triggerLocked
- [x] 5.3 Update worker to call `TriggerAsync` instead of `TriggerNow` for reconcile-triggered gravity
- [x] 5.4 Update `GravityTrigger` interface to add `TriggerAsync(target, reason string)`
- [x] 5.5 Write test: TriggerAsync returns immediately while gravity runs
- [x] 5.6 Write test: duplicate TriggerAsync for same target is deduplicated

## 6. Collector hot-reload

- [x] 6.1 Add `UpdateTargets(targets []config.Target)` method to collector
- [x] 6.2 Call `coll.UpdateTargets()` in watch loop after hot-reload when targets change
- [x] 6.3 Write test: collector updates targets without restart

## 7. Health state metrics

- [x] 7.1 Add `forseti_target_health` gauge to metrics server (labels: target, state)
- [x] 7.2 Add `RecordTargetHealth(target string, state string)` to metrics.Server
- [x] 7.3 Add `RecordTargetHealth` to worker.Recorder interface
- [x] 7.4 Call `RecordTargetHealth` in worker after each reconcile cycle
- [x] 7.5 Write test: health state metric updates on state transitions

## 8. Integration tests

- [x] 8.1 Create `internal/integration/` package with Docker-based test helpers
- [x] 8.2 Write integration test: full reconcile cycle against real Pi-hole container
- [x] 8.3 Write integration test: Pi-hole restart during reconcile — readiness gate catches it
- [x] 8.4 Write integration test: settings change triggers FTL restart, worker recovers
- [x] 8.5 Add `go:build integration` tag so tests don't run in normal `go test ./...`

## 9. Validation

- [x] 9.1 Run `go test ./...` — all tests pass
- [x] 9.2 Run `golangci-lint run` — clean
- [x] 9.3 Run `go vet ./...` — clean
- [x] 9.4 Docker dev smoke test
