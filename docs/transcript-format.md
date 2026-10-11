# Claude Code transcript format (observed)

The format is internal and undocumented. Observed on Claude Code 2.1.268–2.1.296 (October 2026);
every line carries `version`, so parsing rules can be keyed by version. Code that depends on this
lives only in `internal/claudecode`.

## Layout

```
<config>/projects/<project-dir>/<session-id>.jsonl                       main session
<config>/projects/<project-dir>/<session-id>/subagents/agent-<id>.jsonl  subagent runs
<config>/projects/<project-dir>/<session-id>/subagents/workflows/<run>/journal.jsonl
<config>/projects/<project-dir>/memory/                                  auto-memory
```

`<config>` is `~/.claude` or the `CLAUDE_CONFIG_DIR` of another profile. `<project-dir>` is the
lossy encoding of the project path (`claudecode.ProjectDirName`).

## Lines

One JSON object per line; `type` selects the shape. Unknown types must be skipped, not fail.
A truncated last line is possible while a session is being written.

| `type` | Use for quipu |
|---|---|
| `assistant` | token usage, model, cache diagnostics |
| `user` | prompts and tool results; `cwd`, `gitBranch` |
| `attachment` | context injected into the turn (files, reminders, skills) |
| `system` | `subtype`, compaction metadata (`compactMetadata.preTokens/postTokens`) |
| `ai-title`, `agent-name`, `last-prompt` | session title, subagent name |
| `cost-state` | client-side cumulative cost per model (`modelUsage.<model>.*`) |
| others (`mode`, `permission-mode`, `file-history-*`, `queue-operation`, …) | ignore for now |

Common fields on conversation lines: `uuid`, `parentUuid`, `sessionId`, `timestamp` (RFC 3339),
`version`, `cwd`, `gitBranch`, `entrypoint` (`cli`, …), `isSidechain` (true in subagent files),
`sessionKind` (`bg` for background sessions, else absent).

## Assistant usage

```json
{"type":"assistant","message":{"id":"msg_…","model":"claude-opus-5-5","stop_reason":"tool_use",
 "usage":{"input_tokens":2,"cache_read_input_tokens":22685,"cache_creation_input_tokens":24851,
          "output_tokens":177,"cache_creation":{"ephemeral_5m_input_tokens":0,
          "ephemeral_1h_input_tokens":24851},"service_tier":"standard"},
 "diagnostics":{"cache_miss_reason":{"type":"model_changed","cache_missed_input_tokens":51930}}}}
```

**One API response is split across several lines**, one per content block, and every line
repeats the same `usage`. Deduplicate by `message.id` within a file before summing; otherwise
tokens are counted 2–3 times.

Skip `message.model == "<synthetic>"` (client-generated messages, no API call).

Cache hit ratio of a response: `cache_read / (input + cache_read + cache_creation)`.

### Cache miss reasons

`message.diagnostics.cache_miss_reason` is present on a small share of responses and comes from
the API, so it is ground truth rather than inference. Observed `type` values:

| `type` | Meaning |
|---|---|
| `previous_message_not_found` | prefix from the previous request was not in cache (expired or evicted) |
| `messages_changed` | earlier conversation content changed |
| `system_changed` | system prompt changed (CLAUDE.md, settings, output style…) |
| `tools_changed` | tool list changed (MCP server connected, plugin toggled…) |
| `model_changed` | model switched mid-session |

`cache_missed_input_tokens` is present only on some entries.
