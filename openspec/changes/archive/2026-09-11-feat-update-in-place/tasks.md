## 1. ResourceDiff and DiffEntry changes

- [x] 1.1 Add `ActionUpdate DiffAction = "update"` constant to `reconciler.go`
- [x] 1.2 Add `Updates []DiffEntry` field to `ResourceDiff`
- [x] 1.3 Update `HasChanges()` to include `len(d.Updates) > 0`

## 2. Pi-hole client update methods

- [x] 2.1 Add `UpdateAdlist(id int, groups []int) error` method using `PUT /api/lists/<id>`
- [x] 2.2 Add `UpdateClient(id int, groups []int) error` method using `PUT /api/clients/<id>`
- [x] 2.3 Add `UpdateDomain(id int, groups []int) error` method using `PUT /api/domains/<type>/<kind>/<id>` (or appropriate endpoint)
- [x] 2.4 Add unit tests for the new update methods

## 3. PiholeAPI interface extension

- [x] 3.1 Add `UpdateAdlist`, `UpdateClient`, `UpdateDomain` to the `PiholeAPI` interface in `reconciler.go`
- [x] 3.2 Update mock implementations in test files

## 4. Diff function updates

- [x] 4.1 Update `diffAdlists` to accept `groupNameToID map[string]int`, compare resolved group IDs, and emit update entries when groups differ
- [x] 4.2 Update `diffClients` to accept `groupNameToID map[string]int`, compare resolved group IDs, and emit update entries when groups differ
- [x] 4.3 Update `diffDomains` and `diffAllowDomains` to accept `groupNameToID` and detect group changes
- [x] 4.4 Update `Plan()` to pass `groupNameToID` to all updated diff functions
- [x] 4.5 Add helper function for sorted int slice comparison

## 5. Apply logic

- [x] 5.1 Add update processing loop in `Apply()` for adlists (after adds, before deletes)
- [x] 5.2 Add update processing loop in `Apply()` for clients
- [x] 5.3 Add update processing loop in `Apply()` for deny and allow domains

## 6. Tests

- [x] 6.1 Add test cases for adlist group drift detection (changed, identical, order-independent)
- [x] 6.2 Add test cases for client group drift detection
- [x] 6.3 Add test cases for domain group drift detection
- [x] 6.4 Add test case for `HasChanges()` with only updates
- [x] 6.5 Add integration-style test for `Apply()` processing updates

## 7. Verify

- [x] 7.1 Run `go test -race ./...` and confirm all tests pass
- [x] 7.2 Run `go vet ./...` and `golangci-lint run`
