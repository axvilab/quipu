# 0002. DuckDB over Parquet for storage and analytics

Status: Accepted · 2026-10-11

## Context
Claude Code deletes transcripts after 30 days. Transcripts are JSONL, tens of MB per profile;
analytics are column aggregates by project, profile and day.

## Decision
Convert each JSONL transcript once into Parquet under the quipu data dir and query it with
DuckDB through `duckdb-go` (statically linked, requires CGO). Storage sits behind one Go
interface so a pure-Go SQLite backend can replace it if CGO becomes a problem.

## Rejected alternatives
- SQLite as primary: row store, slow wide aggregates.
- Query JSONL directly every time: data disappears after 30 days anyway.

## Consequences
Builds need CGO and a C toolchain; cross-compiling for macOS needs zig or osxcross in CI.
The Makefile currently has `CGO_ENABLED=0` and must change when DuckDB is added.
