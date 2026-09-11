## 1. Group ID translation helper

- [x] 1.1 Add `buildPrimaryIDToName(groups []pihole.APIGroup) map[int]string` helper in `syncer.go`
- [x] 1.2 Add `buildReplicaNameToID(groups []pihole.APIGroup) map[string]int` helper in `syncer.go`
- [x] 1.3 Add `translateGroupIDs(primaryIDs []int, primaryIDToName map[int]string, replicaNameToID map[string]int) []int` helper
- [x] 1.4 Add unit tests for `translateGroupIDs` covering full resolution, partial resolution, and empty input

## 2. Re-fetch replica groups after sync

- [x] 2.1 In `syncOneReplica`, after `syncGroups` completes, re-fetch replica groups via `replica.ListGroups()`
- [x] 2.2 Build the primary-ID→name and replica-name→ID maps from the fetched group lists
- [x] 2.3 Pass both maps to `syncAdlists`, `syncDomains`, and `syncClients`

## 3. Pass translated groups in sync methods

- [x] 3.1 Update `syncAdlists` signature to accept group translation maps, pass translated groups to `CreateAdlist`
- [x] 3.2 Update `syncDomains` signature to accept group translation maps, pass translated groups to `CreateDomain`
- [x] 3.3 Update `syncClients` signature to accept group translation maps, pass translated groups to `CreateClient`

## 4. Tests

- [x] 4.1 Add integration test: adlist synced with correct group IDs on replica
- [x] 4.2 Add integration test: client synced with correct group IDs on replica
- [x] 4.3 Add test: unresolvable group ID is skipped gracefully
- [x] 4.4 Add test: newly created group during syncGroups is resolvable for subsequent resource syncs

## 5. Verify

- [x] 5.1 Run `go test -race ./...` and confirm all tests pass
- [x] 5.2 Run `go vet ./...` and `golangci-lint run`
