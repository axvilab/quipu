# 0005. Prebuilt binaries, install script, user-level autostart

Status: Accepted · 2026-10-11

## Context
quipu must collect transcripts continuously (Claude Code deletes them after 30 days) and the UI
should be one click away. It links DuckDB through CGO, so cross-compiling is painful.

## Decision
- CI builds release binaries natively per platform (linux amd64/arm64, darwin amd64/arm64) on
  each `v*` tag and publishes them with checksums to GitHub Releases.
- `install.sh` downloads the right binary into `~/.local/bin`, verifies the checksum, no root.
- `quipu install` sets up user-level autostart of `quipu daemon`: a systemd user unit on Linux,
  a launchd agent on macOS, plus a menu entry; `quipu uninstall` removes exactly what it created.
- The daemon also serves the UI on a fixed loopback port; `quipu ui` opens the running
  instance using a token the daemon writes to a 0600 file in the data dir.

## Rejected alternatives
- Cross-compiling with zig or osxcross: fragile with DuckDB's C++ build.
- System-wide services or packages needing root: unnecessary for per-user data.
- A separate UI process per `quipu ui`: no stable address, a second launch starts another server.

## Consequences
Release CI needs four runners. The fixed port can be taken; the daemon falls back to a free port
and records the actual address in the same file.
