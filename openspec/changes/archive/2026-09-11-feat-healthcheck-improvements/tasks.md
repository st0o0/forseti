## 1. Healthcheck implementation

- [x] 1.1 Add `--config` flag parsing to the healthcheck case in `cmd/forseti/main.go` (extract to `runHealthcheck(args []string) int`)
- [x] 1.2 Load config via `config.Load()` and iterate over targets
- [x] 1.3 For each target, create a `pihole.Client`, call `Login()` with a 5s context timeout, then `Close()`
- [x] 1.4 Print per-target status lines (`[name] ok` or `[name] error: ...`)
- [x] 1.5 Return exit code 0 if all targets pass, 1 if any fail

## 2. Client timeout support

- [x] 2.1 Check if `pihole.Client.Login()` already supports context; if not, add context propagation or use a goroutine with a timer for the 5s bound

## 3. Dockerfile update

- [x] 3.1 Update Dockerfile HEALTHCHECK CMD to include `--config` flag pointing to the mounted config path

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
