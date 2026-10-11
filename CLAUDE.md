# quipu

Local analyzer for Claude Code: what Claude knows about each project, what it costs in tokens and
cache, duplicated knowledge across projects and profiles, and concrete fixes as diffs.
One Go binary, everything local. Design: `docs/DESIGN.md` (read it before architectural changes).

## Commands

Go and Node run only in Docker; neither is installed on the host. Never call `go` or `npm`
directly.

- `make test`: run all Go tests
- `make vet`, `make fmt`, `make tidy`
- `make web`: build the Svelte frontend (`web/` → `internal/ui/dist/app`)
- `make web-dev`: Vite dev server with hot reload on :5173 (run `quipu ui --port 47801` beside it)
- `make build`: frontend + `bin/quipu`
- Try changes on a fake home, never on the real `~/.claude`: `bin/quipu demo /tmp/qdemo`, then
  `HOME=/tmp/qdemo bin/quipu ui`

## Before you start

- Before a task, check `docs/plans/` for an open plan on the same topic.
- Before an architectural change, read `docs/adr/README.md`. Do not contradict an accepted ADR;
  propose a new ADR that supersedes it instead (`/adr-new`).

## Rules

- Code, comments, docs, commit messages: English.
- All knowledge of Claude Code internals (paths, file formats, transcript schema) lives in
  `internal/claudecode`. Other packages must not hardcode `~/.claude` layout.
- Integrate with Claude Code only through supported means: `CLAUDE_CONFIG_DIR`, settings files,
  files in `.claude/`, plugins, hooks, statusline. Never edit `~/.claude.json`.
- Never read, copy or log credentials. Never put real transcripts or personal paths in test
  fixtures; use synthetic data.
- Tests run against a fake `~/.claude` in `t.TempDir()`.
- Commits: Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`), no AI attribution trailers.
- To commit or push, always follow the `/ship` skill. Never push without the user's confirmation.
