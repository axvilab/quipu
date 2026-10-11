# 0004. Local web UI embedded in the binary

Status: Accepted · 2026-10-11

## Context
The UI must show tables, diffs and token costs, and offer forms for skills and agents.

## Decision
`quipu ui` serves a web UI from `go:embed` over `net/http` on 127.0.0.1, port 0 by default,
with a random token and Host/Origin checks. A second launch opens the running instance.

## Rejected alternatives
- Bubble Tea TUI: poor fit for diffs and forms.
- Electron / Wails from the start: heavier build; Wails may wrap the same UI later.

## Consequences
The binary includes frontend assets. Every write endpoint needs CSRF protection.
