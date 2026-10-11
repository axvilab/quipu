# Live sessions and Remote Control

Status: done · 2026-10-11

## Goal

Show which Claude Code sessions are running now, and loudly which of them have Remote Control
on, so a session left reachable from claude.ai or the phone does not go unnoticed.

## What can be known (Claude Code docs, 2026-10)

- Remote Control lives as long as the local `claude` process; closing the session ends it.
- No supported way lists sessions with Remote Control. `claude remote-control`, `--remote-control`
  and `--rc` are visible in the process's command line; `/remote-control` inside a session is not.
- `CLAUDE_CODE_BRIDGE_SESSION_ID` is set for hook commands while a Remote Control connection is
  active (v2.1.199+).
- `~/.claude/sessions/` is undocumented and holds tokens: quipu does not read it.

## Design

- `claudecode.RunningSessions`: `claude` processes of the user (Linux `/proc`, macOS `ps` and
  `lsof`) with folder, start time and Remote Control from the command line. Helpers that are not
  sessions (`--chrome-native-host`, subcommands like `mcp`, `doctor`, `agents`) are skipped.
- Opt-in hook, from Settings: `quipu hook claude` on SessionStart, UserPromptSubmit, Stop and
  SessionEnd records the session's folder, its `claude` process and whether
  `CLAUDE_CODE_BRIDGE_SESSION_ID` is set, under quipu's data dir. It prints nothing (hook output
  can enter the context) and always exits 0. Installed into the profile's settings.json through
  the journal, so it is undone in one click; other hooks and keys are kept in order.
- Overview: a banner while any session has Remote Control on (where, since when, how to turn it
  off); a list of running sessions; a note when `remoteControlAtStartup` turns it on for every
  session.

## Steps

1. [x] `claudecode`: running sessions (Linux `/proc`, macOS `ps`/`lsof`), settings.json editing
   moved here (advise uses it too) with hook add/remove; tests.
2. [x] `quipu hook claude` and `internal/live` records; tried from inside a real session (found
   its `claude` process, saw the bridge variable).
3. [x] Service + API (`/api/live`, `/api/live/watch`), watch on/off through the journal; tests,
   including a binary path with spaces.
4. [x] UI: banner on every screen, session list on the overview, Settings toggle; uk and en.
5. [x] Tried on the demo home (the real settings.json untouched); docs.
