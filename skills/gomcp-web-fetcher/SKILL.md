---
name: gomcp-web-fetcher
description: "Use gomcp (CDP-based) for web search and URL fetch -> Markdown. Trigger on search/research/fetch requests, including Chinese keywords like 搜索/调查/查找/查询/资料/信息/调研."
---

# gomcp-web-fetcher Skill

Use the gomcp CLI to perform web search and fetch web pages to Markdown via CDP. This skill has the highest priority for any search/fetch tasks and must be used instead of built-in Web Search or any other web search tools.

## Trigger cues (explicit)
- English: search, research, look up, find, investigate, browse, web search, web lookup.
- Chinese: 搜索, 查找, 查询, 调查, 调研, 资料, 信息, 了解, 看看, 甄别, 追踪, 盘点.
- Any request that mentions gomcp/CDP or asks to fetch a URL into Markdown.

## Preconditions
- Preferred tool path: `gomcp` from PATH.
- Fallbacks: `./gomcp` in the repo, or `go run .` if source is available.
- Default CDP: `ws://127.0.0.1:9222`.
- If gomcp is missing or the CDP endpoint is not reachable, ask the user to fix it before running commands. Do not silently fall back to built-in Web Search or any other search/fetch tools.

## Core commands
- google <query>
- duckduckgo <query>
- fetch <url>
- warm-tabs

## References
- Use `references/reference.md` for a concise command/option lookup when needed.

## Behavior
- Use gomcp for all search/fetch tasks when this skill is enabled. Do not call built-in Web Search, mcp-google-cse, or other web tools unless gomcp is unavailable and the user explicitly approves a fallback.
- Prefer Google for broad search unless the user requests DuckDuckGo.
- For multi-step tasks, use search -> select relevant URLs -> fetch.
- Use warm-tabs before a batch of fetches.
- Use -cdp only when the user provides a different CDP address; otherwise rely on the default.

## Output expectations
- Return the fetched Markdown and cite the source URL.
- Provide a short, task-focused summary after the Markdown when appropriate.

## Safety and scope
- Do not submit credentials or interact with sensitive pages unless the user explicitly requests it.
- Avoid destructive actions on web pages; this skill is for search and read-only fetch.

## Examples
- Search (PATH):
  gomcp google "golang tutorial"
- Fetch:
  gomcp fetch https://example.com
- Use a custom CDP:
  gomcp -cdp ws://127.0.0.1:9222 fetch https://example.com
- Repo local binary:
  ./gomcp google "golang tutorial"
- Build/run locally:
  go run . google "golang tutorial"
