# quipu

Local analyzer for Claude Code: what Claude knows about each project, what it costs in tokens and
cache, duplicated knowledge across projects and profiles, and concrete fixes as diffs.
One Go binary, everything local. Design: @docs/DESIGN.md

## Commands

Go runs only in Docker; `go` is not installed on the host. Never call `go` directly.

- `make test`: run all tests
- `make vet`, `make fmt`, `make tidy`
- `make build`: produces `bin/quipu`

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
