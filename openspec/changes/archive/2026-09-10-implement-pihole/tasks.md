## 1. Client Foundation

- [x] 1.1 Define Client struct, NewClient constructor, APIError type
- [x] 1.2 Implement Login (POST /api/auth) and Close (DELETE /api/auth) with idempotent cleanup
- [x] 1.3 Implement internal helper methods: doRequest (with SID header), doJSON (request + decode response)

## 2. Resource CRUD

- [x] 2.1 Implement adlist operations: ListAdlists, CreateAdlist, DeleteAdlists (batch)
- [x] 2.2 Implement domain operations: ListDomains, CreateDomain, DeleteDomains (batch) for deny/allow + exact/regex
- [x] 2.3 Implement group operations: ListGroups, CreateGroup, DeleteGroups (batch)
- [x] 2.4 Implement client operations: ListClients, CreateClient, DeleteClients (batch)
- [x] 2.5 Implement local DNS operations: ListDNSRecords, AddDNSRecord, DeleteDNSRecord, ListCNAMERecords, AddCNAMERecord, DeleteCNAMERecord

## 3. Stats and Actions

- [x] 3.1 Implement GetStats (summary, system info, FTL info) returning a Stats struct
- [x] 3.2 Implement TriggerGravity (POST /api/action/gravity)

## 4. Response Types

- [x] 4.1 Define API response structs: APIList, APIDomain, APIGroup, APIClient, APIDNSRecord, APICNAMERecord, Stats

## 5. Tests

- [x] 5.1 Test auth lifecycle: login, session header sent, close
- [x] 5.2 Test CRUD operations using httptest server with mock responses
- [x] 5.3 Test error handling: non-2xx responses, auth failure, session cleanup on error
- [x] 5.4 Run go vet clean
