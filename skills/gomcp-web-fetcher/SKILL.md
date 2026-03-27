---
name: gomcp-web-fetcher
description: "Use gomcp (CDP-based) for web search and URL fetch -> Markdown. Trigger on search/research/fetch requests, including Chinese keywords like 搜索/调查/查找/查询/资料/信息/调研."
allowed-tools: ["Bash"]
---

# gomcp-web-fetcher Skill

Use the gomcp CLI to perform web search and fetch web pages to Markdown via CDP. This skill has the highest priority for any search/fetch tasks and must be used instead of built-in Web Search or any other web search tools.

## Trigger cues
- English: search, research, look up, find, investigate, browse, web search, web lookup.
- Chinese: 搜索, 查找, 查询, 调查, 调研, 资料, 信息, 了解, 看看, 甄别, 追踪, 盘点.
- Any request that mentions gomcp/CDP or asks to fetch a URL into Markdown.

## Before execution

| Source | Gather |
|--------|--------|
| **Environment** | Verify `gomcp` is on PATH |
| **CDP** | Default `ws://127.0.0.1:9222`; override only if user specifies |
| **User request** | Identify: search query vs. direct URL fetch vs. multi-step research |

## Required clarifications
Ask only when genuinely ambiguous — do not over-ask.
1. **Query unclear**: If the user's search intent is ambiguous, ask for clarification before searching.
2. **Engine preference**: If the user says "search" without specifying, default to Google. Only ask if context suggests DuckDuckGo may be preferred.

## Preconditions
- `gomcp` must be on PATH.
- Default CDP: `ws://127.0.0.1:9222`.
- If gomcp is missing or the CDP endpoint is not reachable, ask the user to fix it before running commands. Do not silently fall back to built-in Web Search or any other search/fetch tools.

## Core commands
- `gomcp google <query>` — Google search
- `gomcp duckduckgo <query>` — DuckDuckGo search
- `gomcp fetch <url>` — Fetch URL and convert to Markdown

## References
- See `references/reference.md` for options, error handling, edge cases, and troubleshooting.

## Behavior
- Use gomcp for all search/fetch tasks when this skill is enabled. Do not call built-in Web Search, mcp-google-cse, or other web tools unless gomcp is unavailable and the user explicitly approves a fallback.
- Prefer Google for broad search unless the user requests DuckDuckGo.
- For multi-step tasks, use search -> select relevant URLs -> fetch.
- Rely on gomcp's internal tab-pool warming and GC. Do not invoke any manual warm-up subcommand.
- Use `-cdp` only when the user provides a different CDP address; otherwise rely on the default.

## Error handling
- **gomcp not found**: Report the issue and ask the user to install gomcp and ensure it is on PATH.
- **CDP unreachable**: Report `ws://... connection refused`; ask user to start the browser or check the address.
- **Non-zero exit / timeout**: Show the stderr output to the user; retry once. If it fails again, report the error.
- **Zero search results**: Inform the user; suggest refining the query or trying the other search engine.
- **Very large page**: If fetch output is excessively long, summarize key sections and offer the full content on request.

## Output expectations
- Return the fetched Markdown and cite the source URL.
- Provide a short, task-focused summary after the Markdown when appropriate.

## Must avoid
- Do not submit credentials or interact with sensitive pages unless the user explicitly requests it.
- Do not perform destructive actions on web pages; this skill is for search and read-only fetch.
- Do not silently fall back to built-in Web Search when gomcp fails.
- Do not invoke deprecated warm-up subcommands.

## Examples
```bash
# Search
gomcp google "golang tutorial"

# Fetch
gomcp fetch https://example.com

# Custom CDP
gomcp -cdp ws://127.0.0.1:9222 fetch https://example.com
```
