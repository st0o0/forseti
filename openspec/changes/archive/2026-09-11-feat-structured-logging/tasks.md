## 1. Config: Add logging fields

- [x] 1.1 Add `LogLevel string` and `LogFormat string` fields to the config struct with yaml tags and defaults (info/text)
- [x] 1.2 Add validation for `log_level` (debug/info/warn/error) and `log_format` (text/json)
- [x] 1.3 Add test cases for valid values, invalid values, and defaults

## 2. Logger initialization

- [x] 2.1 Create slog handler setup in `main.go` based on `cfg.LogLevel` and `cfg.LogFormat`, call `slog.SetDefault()`
- [x] 2.2 Map config string level to `slog.Level` (debug=-4, info=0, warn=4, error=8)

## 3. Migrate log calls

- [x] 3.1 Migrate `cmd/forseti/main.go` log calls to slog with appropriate levels and attributes
- [x] 3.2 Migrate `internal/sync/syncer.go` log calls (16 sites) with target/error/duration attributes
- [x] 3.3 Migrate `internal/gravity/scheduler.go` log calls (5 sites) with target/reason/duration attributes
- [x] 3.4 Migrate `internal/collector/collector.go` log calls (5 sites) with target/error attributes
- [x] 3.5 Remove `"log"` imports from all migrated files, confirm no residual `log.` usage

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
- [x] 4.3 Grep for residual `log.Printf` / `log.Println` — expect zero hits
