## 1. Config model

- [x] 1.1 Add `Kind string` field with `yaml:"kind"` tag to `DenyEntry` and `AllowEntry`
- [x] 1.2 In `applyDefaults()`, set empty `Kind` to `"exact"` for all deny and allow entries
- [x] 1.3 In `validate()`, reject `Kind` values other than `"exact"` and `"regex"`
- [x] 1.4 In `validate()`, use `regexp.Compile` for regex entries instead of `validateDomain`
- [x] 1.5 Add config test cases: default kind, explicit regex, invalid kind, invalid regex pattern

## 2. Reconciler

- [x] 2.1 Collect the set of distinct kinds from deny/allow config entries
- [x] 2.2 Loop over each kind: call `ListDomains(type, kind)`, filter desired entries by kind, diff per-kind
- [x] 2.3 In `Apply()`, pass each entry's kind to `CreateDomain` instead of hardcoded `"exact"`
- [x] 2.4 Update `diffDomains` and `diffAllowDomains` to include kind in diff key if needed
- [x] 2.5 Add reconciler test cases: mixed exact/regex entries, per-kind isolation, regex add/delete

## 3. Sync engine

- [x] 3.1 Update domain sync to iterate over both `exact` and `regex` kinds
- [x] 3.2 Pass the correct kind when creating entries on replicas
- [x] 3.3 Add sync test cases for regex domain propagation

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
