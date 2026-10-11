# Duplicated knowledge across projects

Status: done · 2026-10-11

## Goal

Find the same instructions written more than once and offer a fix as a diff. Two kinds matter:

- **Paid twice in one session**: a project's always-loaded file repeats lines that a user-level
  file (or another always-loaded file of the project) already sends. Removing the copy saves
  tokens in every session of that project.
- **Copied between projects**: the same block of CLAUDE.md, or the same skill, rule or agent,
  lives in several projects. Tokens per session do not change, but every copy has to be kept in
  sync by hand. Moving it to the user's settings keeps one copy (and then loads it in every
  project of that profile, which the finding says).

## Rules

| ID | Finds | Fix |
|---|---|---|
| `repeated-in-session` | lines of a project's always-loaded file that another always-loaded file of the same session already has | remove them from the narrower file (one click) |
| `shared-block` | a block of CLAUDE.md (2+ lines, or one long line) in 3+ projects of the profile | append it to the user CLAUDE.md, remove it from each project (one click, all files or none) |
| `copied-extension` | an identical skill, rule or agent file in 2+ projects | move it to the user's settings and delete the copies (manual: shown with paths) |

Comparison is by normalized lines (trimmed, inner whitespace collapsed, case kept); blank lines,
headings, code fences and lines shorter than 20 characters do not count on their own.

## Steps

1. [x] `advise.Input.Projects`: the items of every known project, scanned once per request.
   Edits gain an `Append` form; a finding's edits are checked together and written only if all
   apply (no half-applied fixes).
2. [x] Rules `repeated-in-session`, `shared-block`, `copied-extension`, with tests on a fake home.
   A block that fills its section moves with its `##` heading; removing a block takes one blank
   line with it.
3. [x] UI: titles and details in uk and en; diffs show every file with line numbers; the changes
   of one fix are one history entry, undone together (journal batches).
4. [x] Demo home triggers all three; checked in the UI (apply, undo); docs.

## Later (done)

- An editor for fixes marked "manual", with the token cost as you type (plan: editor).
- `copied-extension` in one click: a skill moves as its whole folder; the journal deletes files
  with a backup, prunes the folders they leave empty, and removes folders it made on undo. Not
  offered when the user's settings already have one of that name.
- Backups older than 90 days are removed when quipu starts.
