## 1. Config: Add local_dns_purge field

- [x] 1.1 Add `LocalDNSPurge bool` field to the config struct with `yaml:"local_dns_purge"` tag, defaulting to false
- [x] 1.2 Add test cases for config parsing with and without `local_dns_purge`

## 2. Reconciler: Safe DNS diffing

- [x] 2.1 Change `diffDNS` signature to accept a `purge bool` parameter
- [x] 2.2 When `purge` is false, skip populating `Deletes` in `diffDNS`
- [x] 2.3 Update `Plan()` and `Apply()` to pass `cfg.LocalDNSPurge` (or target-level equivalent) to `diffDNS`
- [x] 2.4 Add test cases: additive-only preserves unmanaged records, purge mode deletes them, empty desired with purge deletes all

## 3. Error sentinel fix

- [x] 3.1 Replace `err.Error() != "http: Server closed"` with `!errors.Is(err, http.ErrServerClosed)` in `cmd/forseti/main.go`
- [x] 3.2 Add `"net/http"` import if not already present

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
