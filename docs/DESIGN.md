# quipu — design

> A *quipu* is an Inca record made of knotted cords: knots held the records, cords tied them
> together. quipu does the same for what Claude Code knows across all your projects and accounts.

Status: draft, pre-v0.1. Platforms: **Linux and macOS**. Decisions and their reasons are in
[adr/](adr/README.md); this document describes the system as it is meant to be now.

## 1. Problem

Claude Code loads instructions (CLAUDE.md, rules, skills, memory, plugins) into every session,
and each one costs tokens and cache. Nobody sees this across projects:

- what is loaded always, what on demand, and how much it weighs;
- why the cache misses and where subscription limits go;
- which rules are copied between projects and should live once at user level;
- which knowledge is split across several subscriptions (personal / corporate).

Transcripts, the only record of real usage, are deleted after 30 days.

## 2. Goals / non-goals

Goals:
- Show, per project and profile, what Claude Code loads and what it costs.
- Find repeated and stale knowledge across all projects and profiles.
- Turn every finding into a concrete diff the user can apply or reject.
- Everything local; nothing leaves the machine without an explicit action.

Non-goals:
- Running agents in parallel (Conductor, Claude Squad).
- A vector memory store (Mem0, claude-mem, Basic Memory).
- Rotating accounts to bypass limits; switching API providers (cc-switch).

## 3. Modules and roadmap

| # | Module | Command | Version |
|---|---|---|---|
| 1 | Collector: transcripts → Parquet before the 30-day cleanup; index CLAUDE.md, rules, skills, memory in all profiles | `quipu daemon` | v0.1 |
| 2 | Cache analytics: cache writes, session start weight and hit rate by project/profile/day; API-reported miss reasons | `quipu cache` | v0.1 |
| 3 | Project map: what loads always vs on demand, weight, folders Claude does not see | `quipu map` | v0.1 |
| 4 | Profiles: choose a subscription by folder rule or interactively; shared skills/rules library | `quipu run`, `claude` shim | v0.1 |
| 5 | Advisor: rule-based checks, each finding a diff; later LLM grouping via `claude -p` under the same profile | `quipu advise` | v0.2 |
| 6 | Web UI: editor with token cost, skill/agent forms, move skills between projects | `quipu ui` | v0.2 |
| 10 | Backup: safe subset of `~/.claude` to a trusted private git repo or a local export file, secret scan | `quipu backup`, `quipu restore` | v0.2 |
| 7 | Project switchboard: project × plugin matrix | — | v0.3 |
| 8 | MCP server: search and write knowledge across projects | `quipu mcp` | v0.3 |
| 9 | Team knowledge base: select, anonymize, confirm one by one, git repo | — | v0.4 |
| 11 | New project wizard: scaffold from a stack template, copy skills/rules/agents from other projects | `quipu new` | v0.3 |

First milestone: modules 1–4 as CLI, tested on real data. Continue only if `quipu cache` shows
something useful.

## 4. Architecture

### 4.1 Binary and data

One binary, `quipu`, with subcommands. When invoked as `claude` (symlink) it acts as the profile
shim. Data dir: `~/.local/share/quipu` (Linux), `~/Library/Application Support/quipu` (macOS),
mode 0600, excluded from backups.

### 4.2 Collection and storage

- Sources: `<config>/projects/*/*.jsonl` for every profile config dir (`~/.claude`,
  `~/.claude-<name>`), plus instruction files.
- Each transcript is converted once into Parquet, keyed by profile, project and session; the
  daemon only appends. `SessionEnd` hook gives the path of a finished transcript.
- Queries run in DuckDB over the Parquet files ([ADR 0002](adr/0002-duckdb-parquet.md)).
- Parser is tolerant: the format is internal, each line carries the Claude Code version, rules
  are keyed by version. All format knowledge lives in `internal/claudecode`.
- Optional: do not store message bodies, only metadata and token counts.

### 4.3 Search (later)

Hybrid FTS + cosine similarity over a `FLOAT[384]` column by brute force (DuckDB VSS is
experimental). Local embeddings via fastembed-go, model downloaded on first use. Enabled only if
real data shows FTS misses duplicates.

### 4.4 Project identity

Directory names under `projects/` cannot be decoded reliably. Resolution order:
1. `cwd` from the newest transcripts; prefer their common git root;
2. a path the user confirmed earlier;
3. slug candidates verified against the filesystem;
4. otherwise *unresolved*; never guess silently.

The cross-machine key is the normalized git remote (`host/owner/repo`); worktrees map to the
main repo via `git rev-parse --git-common-dir`.

### 4.5 Profiles and shim

- Default profile is plain `~/.claude` with no env var. Others are `~/.claude-<name>`, exported
  as one canonical absolute `CLAUDE_CONFIG_DIR` (macOS Keychain item name derives from it).
- Shim flow: env already set → run real claude unchanged; non-interactive → pick by rule
  without asking; otherwise ask with a suggestion (folder rule → last used). Run the real claude
  as a child process, forward signals and exit code. The real binary is found on `PATH`,
  skipping our own executable.
- Install method does not matter: native installer (`~/.local/bin/claude` → versions dir, self-
  updating), npm global (deprecated) or Homebrew. Data always lives in the config dir; the shim
  never replaces the binary. `quipu doctor` warns when several installs are on `PATH`.
- Shared library of skills and rules for all profiles; corporate/personal boundary enforced.
- No edits to `.claude.json` ([ADR 0003](adr/0003-supported-integration-only.md)); MCP servers
  are shared through plugins or `.mcp.json`.

### 4.6 Backup (v0.2)

Restores Claude Code setup after an OS reinstall or on a new machine. Two targets, chosen in
quipu settings; both are explicit user actions, never automatic uploads:

- **git repo**: a private repo the user trusts; quipu commits locally, the user (or `quipu backup
  push`) pushes;
- **local export**: one archive file the user stores wherever they back up files.

Included: `CLAUDE.md`, `settings.json`, `rules/`, `skills/`, `agents/`, project memory.
Never included: credentials, `.claude.json`, transcripts, caches, installed plugins.
Per-project exclusions in settings (e.g. work projects whose memory must not leave the machine).
A secret scan runs before every commit or export. `quipu restore` applies a backup with a
preview of every file it will write.

### 4.7 New project wizard (v0.3)

`quipu new` (and "New project" in the UI) creates a project set up the way the advisor would
recommend, instead of fixing it afterwards:

1. Basics: name, folder, stack, docs language, profile (personal / work).
2. Scaffold from a **stack template**: `CLAUDE.md`, `.claude/settings.json`, language rules,
   `adr-new` and `ship` skills, `docs/` (DESIGN, ADR index, plans), `.gitignore`, CI workflow.
3. Pick skills, rules and agents from the user's other projects (known from collection), each
   with its token cost.
4. Preview every file as a diff, then write; optionally `git init` and a first commit.

Stack templates are plain folders of files, not code; later they can be packaged as Claude Code
plugins so the same set can be enabled in existing projects through the switchboard (module 7).

### 4.8 UI

Embedded web UI on 127.0.0.1 ([ADR 0004](adr/0004-embedded-web-ui.md)). Shows when a session is
open: "changes apply after /clear".

### 4.9 Code layout

```
cmd/quipu/           entry point; role chosen by argv[0]
internal/claudecode/ ALL Claude Code internals: paths, dir encoding, transcript schema
internal/collect/    daemon, JSONL → Parquet
internal/store/      storage interface, DuckDB backend
internal/analytics/  cache and cost reports
internal/project/    project identity
internal/profile/    profiles and shim
internal/advise/     rule-based advisor                           v0.2
internal/ui/         web UI                                       v0.2
internal/fsx/        atomic writes, backups
```

## 5. Safety rules

1. LLM analysis of a profile's data runs only under that profile. Only anonymized numbers cross
   profiles.
2. Team base and backups get only selected items; never transcripts or personal memory.
3. Never read, copy or write credentials.
4. Write to `settings.local.json` and `CLAUDE.local.md` by default; shared project files only
   by explicit choice. Every change is a previewed diff with a backup.
5. Re-read before writing files Claude Code itself writes (`settings.json`, `MEMORY.md`).
6. Detect managed settings that forbid hooks or plugins and degrade gracefully.
7. No network access except user-initiated git operations. No telemetry.

## 6. Known limits

- History starts when quipu is installed; older transcripts are already gone.
- Cloud sessions and other machines are invisible; the UI says the picture is partial.
- The API reports a miss reason on only part of the responses; the rest stay unexplained.
- Cache hit rate is 97–99% for typical use (measured 2026-10-11 on real data), so it barely
  separates good from bad setups. What costs limits is **cache writes** (priced above plain
  input) and the **session start weight**: everything sent with the first request. Reports
  lead with these; hit rate is secondary. Baseline is the user's own before/after.

## 7. Prior art (reviewed 2026-10)

| Tool | What it does | Take | Avoid |
|---|---|---|---|
| claude-swap | per-terminal `CLAUDE_CONFIG_DIR`, dir→account map | Keychain naming, lock files | account auto-rotation |
| claude-memory-admin | memory browser, duplicates, health checks | path resolution chain, atomic writes, soft delete | no Host/Origin/CSRF guard |
| claude-code-memory-explorer | memory stack view, LLM triage | finding taxonomy, loopback guard | "read-only" UI that deletes |
| opcode / Claudia | Tauri GUI | session cwd from JSONL | `--dangerously-skip-permissions`, telemetry |

Nobody analyzes cost and duplication across all projects and profiles.

## 8. Engineering

- Go, built and tested in Docker (`make build`, `make test`).
- Tests use a fake `~/.claude` in a temp dir and a fake `claude` binary; no real transcripts.
- CI: GitHub Actions on Ubuntu and macOS. Releases: GoReleaser, Homebrew tap.
- License: MIT.
