# Advisor MVP

Status: done · 2026-10-11

Goal: `quipu advise [path]` with a few rules found on real data, reporting findings and proposed
diffs without applying them. Pulled ahead of profiles (module 4) because it is what sets quipu
apart.

1. [x] Rule framework in `internal/advise`: one rule = one function in its own file, an ID and an
   entry in `Rules`; rules only read; `--disable` skips rules, `--rules` lists them.
2. [x] Rules: `large-import`, `claude-ai-skills`, `long-claude-md`, `memory-limit`.
3. [x] Run on all local projects.

## Outcome

- `claude-ai-skills` fires in every project: it is a user-level problem and should be reported
  once. Needs a cross-project mode (`quipu advise --all`) that groups identical findings by scope.
- `long-claude-md` found two projects with 378 and 497 lines.
- `large-import` would have caught the DESIGN.md import in quipu itself (fixed by hand earlier).

Next for the advisor: `--all` with grouping, applying diffs with a backup, cross-project
duplicate rules.
