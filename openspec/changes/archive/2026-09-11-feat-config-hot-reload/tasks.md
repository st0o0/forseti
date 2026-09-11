## 1. Reload metric

- [x] 1.1 Add `forseti_config_reload_total` counter with `result` label to `internal/metrics/server.go`
- [x] 1.2 Add `RecordConfigReload(success bool)` method to metrics server

## 2. Config reload logic in watch loop

- [x] 2.1 Extract a `tryReloadConfig` helper in `cmd/forseti/main.go` that stats the file, compares mtime, calls `config.Load()`, and returns the new config or nil
- [x] 2.2 Track `lastConfigMtime` (from `os.Stat`) initialized after the initial `config.Load()`
- [x] 2.3 Call `tryReloadConfig` at the top of each ticker case in both config and sync mode branches
- [x] 2.4 On successful reload: swap `cfg` pointer, log the reload, call `srv.RecordConfigReload(true)`
- [x] 2.5 On failed reload: log the error, call `srv.RecordConfigReload(false)`, keep the old config
- [x] 2.6 On successful reload with changed interval: call `ticker.Reset(newInterval)`

## 3. Tests

- [x] 3.1 Add unit test for `tryReloadConfig`: file unchanged returns nil, file changed returns new config, invalid file returns error
- [x] 3.2 Add test for ticker reset logic when interval changes

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
