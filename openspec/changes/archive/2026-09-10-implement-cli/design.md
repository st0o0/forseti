## Context

The CLI has stub commands. All internal packages are ready. This is pure wiring — no new logic, just connecting config loading, pihole clients, reconciler, and metrics server.

## Goals / Non-Goals

**Goals:**
- Working `plan`, `apply`, `watch` commands with `--config` flag
- Colorized diff output for plan/apply
- Graceful shutdown on SIGINT/SIGTERM in watch mode
- Session cleanup guaranteed via defer

**Non-Goals:**
- Interactive mode or TUI
- Config file hot-reload
- Multiple config files

## Decisions

### Use stdlib flag package
No need for cobra/urfave. Three subcommands with one flag each is simple enough for `flag.NewFlagSet`.

### Plan/Apply share the reconcile loop
Both iterate targets, create pihole clients, and call Plan or Apply. The difference is only in what they call and how they display results.

### Watch mode structure
```
main goroutine:
  load config
  start metrics server (goroutine)
  ticker loop:
    for each target:
      create client, apply, record metrics, update stats
  on signal: shutdown metrics, exit
```

### Output format
Plan shows a table per target with adds/deletes/unchanged counts per resource type. Apply shows the same plus success/error status. No JSON output mode for now.

## Risks / Trade-offs
- **No config validation on watch interval** → A very short interval could overload Pi-hole. Acceptable for v1.
