## 1. Docker Compose Setup

- [x] 1.1 Create `docker-compose.dev.yml` with two Pi-hole v6 services (pihole-alpha on 8080/53001, pihole-beta on 8081/53002) and a forseti service using the existing Dockerfile
- [x] 1.2 Create `.env.dev` with default dev passwords for both Pi-hole instances
- [x] 1.3 Add `.env.dev` to `.gitignore` considerations (keep `.env.example` tracked, `.env.dev` tracked with safe defaults)

## 2. Dev Configuration

- [x] 2.1 Create `forseti.dev.yml` with two targets (pihole-alpha, pihole-beta), sample adlists, deny/allow domains, groups, clients, and local DNS entries using env-var expansion for passwords

## 3. Makefile

- [x] 3.1 Create `Makefile` with build targets: `build`, `test`, `lint`, `vet`
- [x] 3.2 Add dev lifecycle targets: `dev-up`, `dev-down`, `dev-logs`, `dev-restart`
- [x] 3.3 Add run targets: `dev-plan`, `dev-apply` that execute forseti against the dev config

## 4. Verification

- [x] 4.1 Run `make dev-up` and verify both Pi-hole instances start and respond on their web UIs
- [x] 4.2 Verify forseti container builds successfully
- [x] 4.3 Run `make build`, `make test`, `make lint` to confirm all build targets work
