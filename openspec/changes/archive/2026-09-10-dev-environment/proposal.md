## Why

There is no local development environment to test Forseti against real Pi-hole instances. Without it, implementing and verifying the config parser, API client, reconciler, and metrics requires either mocking everything or manually setting up Pi-hole instances. A Docker Compose dev setup with two Pi-hole v6 instances enables rapid iteration, multi-target testing, and manual verification via the Pi-hole UI.

## What Changes

- Add `docker-compose.dev.yml` with two Pi-hole v6 containers (pihole-alpha, pihole-beta) and a Forseti build target
- Add `.env.dev` with default development passwords
- Add `forseti.dev.yml` with a sample configuration pointing at both dev Pi-holes, including sample adlists, deny/allow domains, groups, and clients
- Add `Makefile` with targets for dev lifecycle (`dev-up`, `dev-down`, `dev-logs`), build (`build`), test (`test`, `lint`), and run commands (`dev-plan`, `dev-apply`)

## Capabilities

### New Capabilities

None. This change is purely dev infrastructure — no new application capabilities are introduced.

### Modified Capabilities

None. No existing spec requirements change.

## Impact

- New files only: `docker-compose.dev.yml`, `.env.dev`, `forseti.dev.yml`, `Makefile`
- No application code changes
- Requires Docker and Docker Compose on the development machine
- Pi-hole web UIs accessible at `localhost:8080` (alpha) and `localhost:8081` (beta) for manual verification
