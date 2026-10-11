---
name: adr-new
description: Creates a new Architecture Decision Record in docs/adr. Use when a decision that is expensive to reverse has been made (language, storage, data format, security boundary, integration method) or the user asks to "record the decision".
argument-hint: [short decision title]
---

1. Find the highest number in `docs/adr/` and take the next one (4 digits, e.g. `0005`).
2. Create `docs/adr/NNNN-<kebab-slug>.md` from the template below, filled in from the current
   conversation. Keep it to 10–20 lines. Write in English.
3. If it replaces an earlier ADR, set that ADR's status to `Superseded by NNNN` (the only edit
   allowed to an accepted ADR) and put `Supersedes MMMM` in the new one.
4. Add a row to the index table in `docs/adr/README.md`.
5. If the decision changes how the system works, update `docs/DESIGN.md` to match.

Template:

```markdown
# NNNN. $ARGUMENTS

Status: Accepted · YYYY-MM-DD

## Context
What forces the decision; facts, constraints.

## Decision
What we do, in one or two sentences.

## Rejected alternatives
- Option: why not.

## Consequences
What becomes easier, what becomes harder, what we must now watch.
```
