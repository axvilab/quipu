# quipu statusline and shared project memory

Status: done · 2026-10-12 · module 4 (DESIGN §4.5)

## Status line

The dashboard plugin the user had reads `~/.claude/.credentials.json` and calls the usage API, so
under another profile it shows the wrong account's limits. Claude Code already gives a status line
command what it needs on stdin, for the session's own account: `context_window.used_percentage`,
`rate_limits.five_hour` and `seven_day` (Pro and Max only), `prompt_cache`.

- `quipu statusline` reads that JSON and prints one line: profile, context, 5-hour and 7-day use
  with the reset time, cache hit ratio. No store, no network, no credentials; fast.
- Profiles page: per profile, the status line in use (quipu, another command, none) and a switch
  that writes `statusLine` into that profile's `settings.json` through the journal.
- Claude Code runs it at session start, after every reply and when a limit resets.

## Shared project memory

Each profile has its own config dir, so a project moved to the work profile loses the memory it
built under the personal one. Claude Code reads `autoMemoryDirectory` from the project's
`.claude/settings.local.json`, so both profiles can use one memory folder without links.

- For a folder rule, the memory state: shared (and from which profile), or own, with how many
  files another profile has for the project.
- Share: move the files only the rule's profile has into the other profile's memory folder (a file
  of the same name with other content stops it, listed), then set `autoMemoryDirectory` in the
  project's `settings.local.json`. All through one journal batch.
- Stop sharing: copy the shared memory into the rule's profile's own folder, remove the setting.
- Offered when a rule is remembered in the terminal, a switch on the Profiles page, and
  `quipu profile memory share|unshare <folder>`.

## Steps

1. [x] `claudecode`: status line input, profile memory folders; `quipu statusline`; tests.
2. [x] Profiles: status line state and switch; API and UI; uk and en.
3. [x] Shared memory: plan and apply through the journal, CLI, terminal offer, UI; tests.
4. [x] Try on a fake home; docs; v0.7.6.

Tried on a fake home: the status line prints `work · ctx 34% · 5h 24% ↻13:47 · 7d 41% · cache 91%`;
the Profiles page shares memory and turns the status line on; in the terminal a remembered rule is
followed by the offer to share memory. Claude Code honours `autoMemoryDirectory` from
`settings.local.json` under workspace trust, and keys memory by the git root, which is the folder a
remembered rule gets.
