# New project wizard

Status: done · 2026-10-12 · module 11 (DESIGN §4.8)

## Goal

Set up a new or freshly cloned project from the user's own skills, rules and agents, and show what
Claude will read before anything is written.

## What it became, and why

The first version wrote stack templates (rules, commands, permissions), docs skeletons, skills for
ADRs and commits, a CI workflow and ran `git init`. Review found all of it imposed one way of
working: commands are guesses before there is code, not every project uses GitHub, and CI failed
on the empty project. What stays:

- `CLAUDE.md` with the name and purpose (only if the folder has none);
- the user's extensions: copied, or, when identical copies already exist, made global through the
  advice's own fix, so the wizard does not create duplicates;
- an existing folder is fine; existing files are kept;
- the preview of what Claude gets, with token costs;
- a folder rule for the profile.

## Steps

1. [x] `internal/scaffold`: plan, copy extensions, apply through the journal; tests.
2. [x] CLI `quipu new <folder> [--summary] [--from …] [--profile] [--dry-run]`.
3. [x] UI: New project from the overview (`G N`, palette), preview grouped by load; uk and en.
4. [x] Tried on the demo home: new folder with release made global (moved from billing and
   shop-api, one undo restores them), existing folder (all kept); docs.

Found on the way: the store rebuilt its view on every query, and parallel UI requests made DuckDB
fail with a write-write conflict that callers treated as no data; the rebuild is now serialized.
