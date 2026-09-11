## Context

Forseti logs with `log.Printf` using manual `[target]` prefixes and inline format strings. There are ~30 call sites across `cmd/forseti`, `internal/sync`, `internal/gravity`, and `internal/collector`. Go 1.21+ provides `log/slog` in the standard library — no third-party dependency needed.

## Goals / Non-Goals

**Goals:**
- Migrate all `log.Printf` calls to `slog` with appropriate levels
- Support JSON and text output formats via config
- Support configurable log level to control verbosity
- Attach contextual attributes (target name, component, duration) as structured fields

**Non-Goals:**
- Log rotation or file output (container environments use stdout)
- Per-package log level configuration
- Tracing or correlation IDs
- Changing log output in tests (tests can use `slog.SetDefault` with a discard handler)

## Decisions

### 1. Use `log/slog` from stdlib

**Choice**: `log/slog` over `zerolog`, `zap`, or `logrus`.

**Why**: Zero dependencies added. Go 1.26 includes slog. The logging needs (level, format, structured fields) are straightforward — no need for a third-party library.

### 2. Logger initialization in `main.go`

**Choice**: Create a `slog.Handler` (TextHandler or JSONHandler) in `main()` based on config, set it as default with `slog.SetDefault()`. All packages use the package-level `slog.Info/Error/etc.` functions.

**Why over passing logger**: Simpler migration — no need to thread a logger through every struct and function. The global default is sufficient for a single-binary application.

### 3. Log level mapping

| Current pattern | slog level |
|----------------|-----------|
| `log.Printf("...listening on...")` | `slog.Info` |
| `log.Printf("...reconciled in...")`, `...synced...`, `...gravity completed...` | `slog.Info` |
| `log.Printf("...warning:...")` | `slog.Warn` |
| `log.Printf("...error:...")`, `...session error...` | `slog.Error` |
| `log.Println("shutting down...")` | `slog.Info` |
| `log.Printf("...scheduled, next run...")` | `slog.Debug` |

### 4. Config fields

```yaml
log_level: info    # debug | info | warn | error
log_format: text   # text | json
```

Both are top-level fields. Defaults: `info` / `text`. Validated in `validate()`.

## Risks / Trade-offs

- **30+ call sites to migrate** → Mechanical but broad change. Risk of missing a call site mitigated by grepping for residual `log.` imports after migration.
- **Global logger** → Sufficient for this application. If forseti ever becomes a library, the logger should be injected — but that's not a current concern.
- **Log output format changes** → Existing log parsers or grep patterns against `[target]` prefix format will break. Mitigated by release notes.
