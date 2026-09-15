# Changelog

## [0.1.8](https://github.com/st0o0/forseti/compare/v0.1.7...v0.1.8) (2026-09-15)


### Bug Fixes

* make gravity scheduler non-blocking with async goroutine execution ([0d29c58](https://github.com/st0o0/forseti/commit/0d29c58a34227ecc98aaf380be5fc447e8e22fb3))
* reset drift gauge to zero when resource type returns to sync ([ff20e4e](https://github.com/st0o0/forseti/commit/ff20e4e4fe0f28e2bd53cecbec3d2ad98b235a6c))

## [0.1.7](https://github.com/st0o0/forseti/compare/v0.1.6...v0.1.7) (2026-09-15)


### Features

* migrate to modular build and docker workflows ([afbec78](https://github.com/st0o0/forseti/commit/afbec78a697142074c5bee1b9c55ed545c4a3507))
* migrate to multi-stage Dockerfile ([a773e77](https://github.com/st0o0/forseti/commit/a773e77fb4e0fcba95aff9eb08037704f431918d))


### Bug Fixes

* add id-token permission for cosign signing in dev builds ([c9087cb](https://github.com/st0o0/forseti/commit/c9087cbb4f2c6bc2a8837e381020ec5260f8e783))
* include user comment in API calls instead of bare marker ([b5733b8](https://github.com/st0o0/forseti/commit/b5733b8f0a1edd5250b6cfcfdf086d8d2b797d36))

## [0.1.6](https://github.com/st0o0/forseti/compare/v0.1.5...v0.1.6) (2026-09-13)


### Documentation

* add per-target API concurrency gate to README ([c1a2690](https://github.com/st0o0/forseti/commit/c1a2690ce607934d772ddc11430f0730e55ac8f7))

## [0.1.5](https://github.com/st0o0/forseti/compare/v0.1.4...v0.1.5) (2026-09-13)


### Features

* add per-target API concurrency gate and configurable timeouts ([5e4da90](https://github.com/st0o0/forseti/commit/5e4da90a51b6537de6a2c8db4b9ede0571adee88))
* **pihole:** Add API settings to targets ([269a11d](https://github.com/st0o0/forseti/commit/269a11d75d21302d9d50d696fa93489ba252a772))

## [0.1.4](https://github.com/st0o0/forseti/compare/v0.1.3...v0.1.4) (2026-09-12)


### Features

* add error classification, retry logic, and transient fault handling ([7db3679](https://github.com/st0o0/forseti/commit/7db367969af587885dd664851fca1f496043ad97))
* add per-target worker architecture with health metrics and readiness checks ([e7d2950](https://github.com/st0o0/forseti/commit/e7d29505703d3c79a40af0cd6852c9ec2935f59f))
* prevent gravity loop with in-flight tracking and async triggers ([b938927](https://github.com/st0o0/forseti/commit/b938927464d398627cb8a5aefb4570ff554ec7c6))


### Documentation

* add openspec specs for worker, error resilience, gravity loop prevention, and backoff ([85044b3](https://github.com/st0o0/forseti/commit/85044b3d1a5c496999782f6373110f78c25e2ca0))

## [0.1.3](https://github.com/st0o0/forseti/compare/v0.1.2...v0.1.3) (2026-09-12)


### Bug Fixes

* invalidate stale sessions on target failure to allow recovery ([4c3d24b](https://github.com/st0o0/forseti/commit/4c3d24b552f95ea4c96804e58e3fff10c4569eae))
* wait for FTL ready after settings changes to prevent connection refused ([d4ce9ba](https://github.com/st0o0/forseti/commit/d4ce9bade5afa0b73ddf376e16c00fb3d244ac92))

## [0.1.2](https://github.com/st0o0/forseti/compare/v0.1.1...v0.1.2) (2026-09-12)


### Bug Fixes

* check error returns in merge test to satisfy errcheck linter ([57f7ce9](https://github.com/st0o0/forseti/commit/57f7ce9ec89075329139cb1d5ff26eeef01fe776))
* check remaining errcheck violations in merge test ([b882729](https://github.com/st0o0/forseti/commit/b882729606fa55eef2656f49b7ad4b6c69f4ee12))
* settings reconciliation bugs and API path corrections ([744b435](https://github.com/st0o0/forseti/commit/744b4354d2f5685aaf9d857a9f5216070e84a064))

## [0.1.1](https://github.com/st0o0/forseti/compare/v0.1.0...v0.1.1) (2026-09-12)


### Features

* decouple release-please from build workflow ([be61b96](https://github.com/st0o0/forseti/commit/be61b9687a79ffbd91640c2f55b1c621e6e205ed))


### Bug Fixes

* bool-to-int coercion and collect-and-continue in settings reconciliation ([3764321](https://github.com/st0o0/forseti/commit/376432153577a6041d1966623353479145c14cc6))

## 0.1.0 (2026-09-11)


* remove dependabot in favor of renovate ([6371dd1](https://github.com/st0o0/forseti/commit/6371dd1e4cce57f32d50167c8c38be901ea78546))


### Features

* add CNAME record reconciliation with additive-only default ([fdde974](https://github.com/st0o0/forseti/commit/fdde974cccc542b67099124a36a5562bb3a234d4))
* add collector toggles, DHCP metrics, and extended Prometheus metrics ([73bba0f](https://github.com/st0o0/forseti/commit/73bba0f0c4ea3492f6be9b504891a6591761ba45))
* add config parser, Pi-hole v6 API client, and session pool ([32a31a4](https://github.com/st0o0/forseti/commit/32a31a43053662bebe6192127ac501d87998d031))
* add project scaffolding, CI, and documentation ([17244b8](https://github.com/st0o0/forseti/commit/17244b832085a9c2daad212a2f65b3a4e0963194))
* add Prometheus metrics, stats collector, gravity scheduler, and CLI ([151720c](https://github.com/st0o0/forseti/commit/151720c576fb6680338287462b0a96753aceca5c))
* add reconcile diff engine and primary-to-replica sync ([7c6b34c](https://github.com/st0o0/forseti/commit/7c6b34c82aa6871048da8b647eab83d04852de40))
* add regex/wildcard domain support for deny and allow entries ([2b0d7c2](https://github.com/st0o0/forseti/commit/2b0d7c202c4be21ead055fb436828549b71c49dc))
* add settings reconciliation and per-target config overrides ([499f9cc](https://github.com/st0o0/forseti/commit/499f9cc791033b7c8a37eff156d2026b00cd66be))
* config hot-reload in watch mode via mtime polling ([c13ae07](https://github.com/st0o0/forseti/commit/c13ae0778ea21a08cf7b158f0bf84875ee2ad44c))
* healthcheck verifies Pi-hole target connectivity with timeout ([a3634ed](https://github.com/st0o0/forseti/commit/a3634edaa8e0470b7443c2d5ccff0c600fe9b624))
* migrate all logging to log/slog with configurable level and format ([0c878c6](https://github.com/st0o0/forseti/commit/0c878c625bfd42ecd5b3e2d877f7218209669daa))
* preserve group assignments during primary-to-replica sync ([e10b112](https://github.com/st0o0/forseti/commit/e10b112818f296ea7e2ac3fac24c87c25a951283))
* update-in-place diffing for adlist and client group assignments ([8adebf1](https://github.com/st0o0/forseti/commit/8adebf19db9f4404995e774a366849589c993107))


### Bug Fixes

* check all error return values in test files (errcheck) ([b73c5df](https://github.com/st0o0/forseti/commit/b73c5df8a905aec187ff3ac6b5d1fa477b5991fa))
* gravity scheduler fires immediately on impossible cron expressions ([8776dd4](https://github.com/st0o0/forseti/commit/8776dd450d18e59d495e314f607759a66928b74b))
* Pi-hole v6 API compatibility for resource updates and settings ([393a9e5](https://github.com/st0o0/forseti/commit/393a9e58619b1c4cb31067013290fb5540c83fc0))
* resolve data race in metrics and fix all golangci-lint errors ([2531171](https://github.com/st0o0/forseti/commit/25311716bebec365fb6042bca67892c10ab06f3a))
* safe DNS reconciliation with additive-only default and error sentinel ([0ed4117](https://github.com/st0o0/forseti/commit/0ed4117f7fccdc8ece7241c1adce5a46c463e98d))


### Documentation

* add OpenSpec specifications and archived change artifacts ([8d57bfd](https://github.com/st0o0/forseti/commit/8d57bfd4801808a25983dbd6260b59535035e5b7))
* rewrite README with full configuration reference ([13cb232](https://github.com/st0o0/forseti/commit/13cb23280d7497de6da3458946a646278059d89e))
* sync 9 new capability specs to openspec/specs ([debb21a](https://github.com/st0o0/forseti/commit/debb21a026f7efeaa802dfe81bbe9bc22c68f8be))


### Refactoring

* improve logging consistency and add debug logging ([e702779](https://github.com/st0o0/forseti/commit/e702779fe9acf307502f5eb009b9d4e5dd93bc6d))
* rename CI jobs for cleaner GitHub check names ([eb5d65c](https://github.com/st0o0/forseti/commit/eb5d65c251764103ad0ea95541bd6c1f10dcd376))
