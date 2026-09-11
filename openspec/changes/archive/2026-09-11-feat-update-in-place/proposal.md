## Why

The reconciler currently only detects add and delete operations. When a managed adlist's group assignments change, or a client's groups change in the YAML config, the existing entry stays unchanged on the Pi-hole instance. The only workaround is to delete and recreate the entry, which loses Pi-hole-side state (hit counters, last-seen timestamps). True update-in-place diffing is needed to detect when a key matches but attributes differ.

## What Changes

- **Update detection in diff functions**: `diffAdlists` and `diffClients` will compare group assignments between desired and actual state. When the key matches but groups differ, the entry is classified as an update rather than unchanged.
- **New `Updates` field on `ResourceDiff`**: A new diff action type alongside adds and deletes, carrying both the key and the resource ID needed for the API call.
- **Pi-hole client update methods**: New `UpdateAdlist` and `UpdateClient` methods using PUT requests to the Pi-hole v6 REST API.
- **`PiholeAPI` interface extension**: New methods added to the reconciler's API interface.
- **Apply logic**: Process updates after adds, calling the new update methods with resolved group IDs.
- **Domain group updates**: `diffDomains` and `diffAllowDomains` will also detect group assignment changes for deny/allow domains that support groups.

## Capabilities

### New Capabilities

- `reconcile-updates`: Update-in-place diffing and application for adlists, clients, and domains when group assignments change.

### Modified Capabilities

_(none — no existing specs)_

## Impact

- `internal/pihole/client.go`: new `UpdateAdlist`, `UpdateClient`, `UpdateDomain` methods
- `internal/reconcile/reconciler.go`: new `ActionUpdate` const, `Updates` field on `ResourceDiff`, updated diff functions and `Apply()` logic
- `internal/reconcile/reconciler_test.go`: new test cases for update detection
- `PiholeAPI` interface: new methods (breaking for any external implementors, but the interface is internal)
