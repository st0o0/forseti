## 1. Async gravity execution

- [x] 1.1 Add `sync.WaitGroup` field to `Scheduler` struct
- [x] 1.2 Refactor `triggerLocked` to set `inFlight` under lock, then spawn a goroutine for I/O (pool.Acquire, pool.Get, TriggerGravity, RecordGravityRun) with `wg.Add(1)` / `defer wg.Done()`; goroutine clears `inFlight` under lock when done
- [x] 1.3 In `Start`, add `wg.Wait()` after the event loop exits on `ctx.Done()`

## 2. Simplify processAsync

- [x] 2.1 Refactor `processAsync` to use async `triggerLocked` for targets with schedule entries (remove redundant manual unlock/relock path)
- [x] 2.2 For targets without a schedule entry, create a helper that follows the same async pattern as `triggerLocked`

## 3. Update tests

- [x] 3.1 Update `TestTriggerSuccessful`, `TestTriggerWithSessionError`, `TestTriggerGravityError`, `TestTriggerWithNilRecorder` — these call `triggerLocked` directly and expect synchronous completion; add a short wait or use WaitGroup
- [x] 3.2 Add test: verify event loop remains responsive while gravity blocks (start scheduler, block gravity handler, verify a second target's cron trigger fires without waiting)
- [x] 3.3 Add test: verify graceful shutdown waits for in-flight gravity

## 4. Verify

- [x] 4.1 Run `go test -count=1 ./internal/gravity/...` and `go test ./...`
- [x] 4.2 Run `golangci-lint run`
- [x] 4.3 Docker test: rebuild and verify with `docker-compose.gravity-test.yml` — confirm no `database is locked` errors with concurrent gravity + reconcile
