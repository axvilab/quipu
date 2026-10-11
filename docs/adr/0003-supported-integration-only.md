# 0003. Integrate with Claude Code only through supported means

Status: Accepted · 2026-10-11

## Context
Claude Code internals change fast. The first design mirrored `mcpServers` into each profile's
`.claude.json`, a file Claude Code owns and rewrites.

## Decision
quipu uses only `CLAUDE_CONFIG_DIR`, `autoMemoryDirectory`, settings files, files in `.claude/`,
plugins with its own marketplace, hooks (`SessionEnd`) and the statusline. It never edits
`~/.claude.json`. Transcripts are read-only input, parsed tolerantly per Claude Code version.

## Rejected alternatives
- Editing `.claude.json` (MCP mirroring): races with Claude Code, breaks on format changes.

## Consequences
Sharing MCP servers across profiles must go through plugins or `.mcp.json`. All format
knowledge stays in `internal/claudecode`.
