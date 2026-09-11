## 1. Interval validation

- [x] 1.1 Add minimum interval constants to `internal/config/config.go`
- [x] 1.2 Add interval validation checks in `validate()` for reconcile.interval, sync.interval, and metrics.scrape_interval
- [x] 1.3 Add test cases for negative intervals, sub-minimum intervals, and valid intervals

## 2. Gitignore

- [x] 2.1 Create `.gitignore` with entries for `*_cov.out`, `coverage/`, Go binary, and editor files
- [x] 2.2 Remove untracked coverage files from working directory

## 3. Verify

- [x] 3.1 Run `go test -race ./...` and confirm all tests pass
- [x] 3.2 Run `go vet ./...` and `golangci-lint run`
