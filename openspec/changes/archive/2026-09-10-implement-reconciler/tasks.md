## 1. Core Types and Interface

- [x] 1.1 Define PiholeAPI interface with the methods needed by the reconciler
- [x] 1.2 Define DiffReport, ResourceDiff, and ApplyReport types
- [x] 1.3 Define DiffAction (add/delete) and DiffEntry types for tracking individual changes

## 2. Diff Logic

- [x] 2.1 Implement group diff: match by name, filter by marker
- [x] 2.2 Implement adlist diff: match by URL, filter by marker
- [x] 2.3 Implement domain diff (deny/allow): match by domain, filter by marker
- [x] 2.4 Implement local DNS diff: match by domain+IP pair
- [x] 2.5 Implement client diff: match by IP/CIDR, filter by marker

## 3. Plan and Apply

- [x] 3.1 Implement Plan function: fetch actual state, compute diff for all resource types, return DiffReport
- [x] 3.2 Implement Apply function: compute diff, execute adds/deletes in order, trigger gravity if needed, return ApplyReport
- [x] 3.3 Implement multi-target Reconcile function that runs Plan or Apply per target independently

## 4. Tests

- [x] 4.1 Create mock PiholeAPI implementation for testing
- [x] 4.2 Test three-way diff: desired [A,B,C] vs actual [A,C,D(managed),E(unmanaged)] -> add B, delete D, leave E
- [x] 4.3 Test reconcile ordering: groups before dependents
- [x] 4.4 Test gravity trigger only on adlist changes
- [x] 4.5 Test multi-target: one target fails, other succeeds
- [x] 4.6 Run go vet clean
