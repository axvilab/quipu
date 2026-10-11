# Architecture decisions

One file per decision that is expensive to reverse. Accepted ADRs are never edited, except to
mark them superseded; a changed decision gets a new ADR. Create one with `/adr-new`.

| # | Decision | Status |
|---|---|---|
| [0001](0001-analyzer-first.md) | The multi-project analyzer is the core of quipu | Accepted |
| [0002](0002-duckdb-parquet.md) | DuckDB over Parquet for storage and analytics | Accepted |
| [0003](0003-supported-integration-only.md) | Integrate with Claude Code only through supported means | Accepted |
| [0004](0004-embedded-web-ui.md) | Local web UI embedded in the binary | Accepted |
| [0005](0005-distribution-and-autostart.md) | Prebuilt binaries, install script, user-level autostart | Accepted |
| [0006](0006-svelte-spa.md) | Svelte single-page UI embedded in the binary | Accepted |
| [0007](0007-native-window-wails.md) | Native window with Wails in a separate binary | Accepted |
