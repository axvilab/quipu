# 0006. Svelte single-page UI embedded in the binary

Status: Accepted · 2026-10-11

## Context
The first UI was server-rendered `html/template` pages (UI MVP plan). It works but feels raw:
every action reloads the page, and the planned features (command palette, keyboard navigation,
detail panels, the project switchboard, a CLAUDE.md editor with live token cost) are interactive.
The UI must also speak Ukrainian and English.

## Decision
The UI becomes a Svelte 5 single-page app built with Vite and Tailwind 4 into `internal/ui/dist`,
embedded with `go:embed` as before (ADR 0004 still holds: one binary, loopback only). The Go server
serves the app and a JSON API under `/api/`; all logic, file edits and safety checks stay in Go.
Texts live in `web/src/locales/{uk,en}.json`. Fonts are bundled; the UI makes no network requests.
Node runs only for development (in Docker, like Go) and in CI; users never need it.

## Rejected alternatives
- Keep `html/template` and add JavaScript by hand: possible, but each interactive feature would be
  hand-rolled DOM code.
- SvelteKit: routing and SSR we do not need; plain Svelte with a small router is enough.
- React/Vue: larger runtime; no advantage here.

## Consequences
Builds need Node (a Docker image locally, setup-node in CI) and `npm ci` before `go build`; a
placeholder page is committed so `go build` works without the frontend. npm dependencies are kept
to a minimum and need a reason.
