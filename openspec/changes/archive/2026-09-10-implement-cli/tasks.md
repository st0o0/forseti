## 1. Flag Parsing

- [x] 1.1 Implement --config flag parsing for plan, apply, and watch subcommands using flag.NewFlagSet

## 2. Plan Command

- [x] 2.1 Implement plan: load config, iterate targets, call Plan, print diff report per target

## 3. Apply Command

- [x] 3.1 Implement apply: load config, iterate targets, call Apply, print apply report per target

## 4. Watch Command

- [x] 4.1 Implement watch: load config, start metrics server, run reconcile loop on interval
- [x] 4.2 Add signal handling for graceful shutdown (SIGINT/SIGTERM)
- [x] 4.3 Add stats scraping in the reconcile loop (UpdateStats per target)

## 5. Output Formatting

- [x] 5.1 Implement diff report printer showing adds/deletes/unchanged per resource type per target

## 6. Verification

- [x] 6.1 Build binary and test plan/apply/watch --help output
- [x] 6.2 Run go vet and full test suite
