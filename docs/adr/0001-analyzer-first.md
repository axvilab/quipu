# 0001. The multi-project analyzer is the core of quipu

Status: Accepted · 2026-10-11

## Context
The first design put subscription profiles and a git memory vault at the center. Reviewing
existing tools (2026-10-10) showed profile switchers already exist, while nobody analyzes what
Claude Code loads across all projects and profiles, what it costs, and where knowledge repeats.

## Decision
quipu is first an analyzer: collect transcripts and instruction files from every project and
profile, report token and cache cost, find duplicates, and propose fixes as diffs. Profiles,
the editor, the project switchboard and backups are conveniences around it.

## Rejected alternatives
- Profiles + vault as the core: crowded space, small moat.
- A vector memory store: Mem0, claude-mem, Basic Memory exist.

## Consequences
v0.1 ships collection, cache analytics and a project map before any write features.
The old vault design is postponed (backup module, v0.2).
