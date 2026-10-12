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
| 5 | Advisor: rule-based checks (one rule per file, easy to add), each finding a diff; duplicated instructions within a session and across projects | `quipu advise` | v0.1 (MVP), v0.2 |
| 6 | Web UI: editor with token cost, skill/agent forms, move skills between projects | `quipu ui` | v0.2 |
| 10 | Backup: safe subset of `~/.claude` to a trusted private git repo or a local export file, secret scan | `quipu backup`, `quipu restore` | v0.2 |
| 7 | Project switchboard: project × plugin matrix, plugin catalog and stack-based recommendations, install via `claude plugin` | `quipu plugins` | v0.3 |
| 8 | MCP server: search and write knowledge across projects. Postponed: its tool definitions would sit in every session, the kind of cost quipu removes, and the gain is unclear | `quipu mcp` | later |
| 9 | Team knowledge base: select, anonymize, confirm one by one, git repo | — | v0.4 |
| 11 | New project wizard: scaffold from a stack template, copy skills/rules/agents from other projects | `quipu new` | v0.7 |

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
- The web interface collects in the background, at start and then every minute; pages read what
  is stored and never wait for a run. Only the very first run on an empty store shows a progress
  screen. One run at a time across the daemon, the interface and the CLI; a run that stored
  something new bumps a generation, and open pages load again.
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
- `quipu doctor` does not re-implement Claude Code's own `/doctor`, which audits one install
  interactively; it checks what quipu sets up and depends on, across profiles.
- Shared library: at `quipu profile add` the new profile links to `~/.claude/{CLAUDE.md,skills,
  agents,rules,commands}`; `projects/` (memory, transcripts) is never shared, which keeps the
  personal / work boundary. Login state comes from `claude auth status --json`.
- `quipu run` with folder rules (`quipu profile rule <folder> <name>`), and the `claude` shim:
  a link `claude` → quipu in quipu's own folder, put first on PATH by one line in the shell's rc
  file (`quipu profile shim on --rc`, or Settings). The real claude is never replaced.
- `quipu doctor` (and the Check section in Settings): Claude Code found and its installs, profile
  logins, broken shared links, stale folder rules, the shim, the service, the window.
- No edits to `.claude.json` ([ADR 0003](adr/0003-supported-integration-only.md)); MCP servers
  are shared through plugins or `.mcp.json`.

In a folder without a rule, `quipu run` (and the shim) asks which account to use and offers to
remember it as a rule for the git repository the folder is in, or the folder itself (never for
home, a folder above it, the root or the temp folder; Enter means yes). Every run under a rule
prints one line naming the profile and the rule, so a wrong one is noticed at once.

Each profile keeps its own project memory, so a project moved to another account would lose what
Claude learned there. For a folder rule, quipu can share it: the files only the rule's profile has
move into the other profile's memory folder, and the project's `.claude/settings.local.json` sets
`autoMemoryDirectory` to that folder, which Claude Code reads under every profile (a supported
setting, honoured once the folder is trusted). One journal batch; stopping gives the profile a copy.
It is offered when a rule is remembered in the terminal, on the Profiles page and with
`quipu profile memory`.

`quipu statusline` is a status line built from what Claude Code passes it for the session's own
account: profile, context, 5-hour and 7-day limits with the reset, cache hit ratio. No store,
network or login is read. A switch per profile on the Profiles page writes `statusLine`.

The PATH line goes into the login shell's startup file; Settings lists bash, zsh and fish, so a
terminal that runs another shell (an IDE's, often zsh) can be connected on its own. Turning the
shim off removes the line from every shell.

### 4.6 Backup

Restores Claude Code setup after an OS reinstall or on a new machine (`quipu backup`, `quipu
restore`, the Backup page; plan: `plans/2026-10-11-backup.md`). Two targets; both are explicit
user actions, never automatic uploads:

- **git repo**: a private repo the user trusts; quipu commits locally, the user (or `quipu backup
  push`) pushes;
- **local export**: one archive file the user stores wherever they back up files.

Included: `CLAUDE.md`, `settings.json`, `rules/`, `skills/`, `agents/`, project memory.
Never included: credentials, `.claude.json`, transcripts, caches, installed plugins.
Per-project exclusions in settings (e.g. work projects whose memory must not leave the machine).
A secret scan runs before every commit or export. `quipu restore` applies a backup with a
preview of every file it will write.

### 4.7 Plugins

Done: per project, which plugins are enabled where, what they list in every session and when they
were last used (from transcripts); rules for plugins unused in 30 days and plugins enabled for
every project but used in one (plan: `plans/2026-10-11-plugins.md`). Planned:

quipu never installs plugin files itself; it runs `claude plugin install|enable|disable … --scope
project|local|user` (with the profile's `CLAUDE_CONFIG_DIR`), so the Claude Code install method
does not matter, only a version that has these commands.

- Postponed: matching plugins to a project's stack by keywords would guess more than it knows.
- Catalog: marketplaces the user already added (read from the config dir, read-only).
- Recommendations by detected stack (`go.mod`, `composer.json`, `package.json`, …), each with
  its per-request token cost from `claude plugin details`.
- Default scope is project or local: an enabled plugin is paid in every session.
- Before install, show what the plugin contains (skills, agents, hooks, MCP servers); hooks and
  MCP run with the user's rights. Only trusted marketplaces are suggested by default.
- Advisor rule: a plugin enabled for the user but used in one project should move to project scope.

A Plugins page lists every plugin installed in every profile (`installed_plugins.json`): whether
it is on for every project, only in some, or off; what it lists in each session; its use in the
last 30 days under that profile. Turning one on or off edits the profile's `enabledPlugins`
through the journal; removing runs `claude plugin uninstall` under that profile's config dir, with
a confirmation, since quipu cannot undo it.

### 4.8 New project wizard

`quipu new` (and New project on the overview, `G N`) sets up a new or freshly cloned folder from
the user's own work, without imposing a way of working:

1. Folder (new, or existing: files already there are never touched), purpose, profile.
2. Skills, rules and agents from the user's other projects, each with what it adds to every
   session; the most recently changed copy when versions differ. What is already identical in
   several projects is offered as **global** (moved to the user's settings by the advice's
   `copied-extension` fix) instead of copied again, so the wizard never makes the duplicates the
   advice warns about.
3. Preview "what Claude gets": files grouped by when Claude reads them, each explained and with
   its token cost, including what the folder already has; the session start compared with the
   user's other projects. Costs come from laying the files out in a scratch folder and reading it
   with `claudecode.Instructions`, as for a real project. Then one journal batch for the files and
   any move to global, so one undo reverts both; a folder rule for the profile.

Written by quipu itself: only a `CLAUDE.md` with the name and purpose, if the folder has none. No
stack templates, permissions, docs skeletons, CI or `git init`: commands are guesses before there
is code (`/init` writes them from the real project), not every project lives on GitHub, and a
way of working is the user's own to bring, as their skills.

### 4.9 UI

Embedded web UI ([ADR 0004](adr/0004-embedded-web-ui.md)), served by the daemon on a fixed loopback
port ([ADR 0005](adr/0005-distribution-and-autostart.md)) at `http://quipu.localhost:47800`:
browsers and systemd-resolved map `*.localhost` to loopback, so no `/etc/hosts` edit. Listens on
127.0.0.1 and ::1; other Host headers are refused. `quipu ui` opens it in the Quipu window
([ADR 0007](adr/0007-native-window-wails.md)): `quipu-app`, a small Wails binary with its own
icon that forwards the webview's requests to the server (no data, no port of its own; the token
comes in an environment variable). Only one window runs at a time. Without the window (no WebKit
on Linux, or `--browser`) it opens a browser app window (Chrome/Chromium/Edge/Brave `--app`) or a
tab. Shows when a session is open: "changes apply after /clear". Theme (system, light, dark) and
language are per window, on the settings page.

Live sessions: the overview lists the running Claude Code sessions (the user's `claude`
processes), and a banner on every screen says when one has Remote Control on. Sessions started
with `--rc` or `remote-control` are seen from their command line; `/remote-control` inside a session
only through an opt-in hook (`quipu hook claude`, added to the profile's settings.json from
Settings, through the journal) that records `CLAUDE_CODE_BRIDGE_SESSION_ID` on session events.
`~/.claude/sessions/` is not read: it is undocumented and holds tokens.

Install: `quipu install` adds a systemd user unit (Linux) or launchd agent (macOS) for the daemon
and a menu entry with the icon (Linux; on macOS install.sh puts Quipu.app into ~/Applications);
`quipu uninstall` removes exactly what it created.

### 4.10 Code layout

```
cmd/quipu/           entry point; role chosen by argv[0]
internal/claudecode/ ALL Claude Code internals: paths, dir encoding, transcript schema
internal/collect/    daemon, JSONL → Parquet
internal/store/      storage interface, DuckDB backend
internal/analytics/  cache and cost reports
internal/project/    project identity
internal/profile/    profiles and shim
internal/scaffold/   setting up a project folder
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
