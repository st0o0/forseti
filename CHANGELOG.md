# Changelog

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
