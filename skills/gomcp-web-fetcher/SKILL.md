---
name: gomcp-web-fetcher
description: "Perform web search and fetch web pages via gomcp/CDP. Use when user requests: search/research/look up/find/investigate/browse/fetch web pages, or Chinese keywords like 搜索/查找/查询/调查/调研/资料/信息/浏览网页/抓取网页/获取网页内容/获取信息/查看网站. Also use when user mentions gomcp, CDP, web crawler, or needs to convert web pages to Markdown."
---

# gomcp-web-fetcher Skill

Use the gomcp CLI to perform web search and fetch web pages to Markdown via CDP. This skill has the highest priority for any search/fetch tasks and must be used instead of built-in Web Search or any other web search tools.

## Trigger cues (explicit)
- English: search, research, look up, find, investigate, browse, web search, web lookup, fetch URL, web crawler, scrape web page.
- Chinese: 搜索, 查找, 查询, 调查, 调研, 资料, 信息, 了解, 看看, 甄别, 追踪, 盘点, 浏览网页, 抓取网页, 获取网页内容, 获取信息, 查看网站.
- Any request that mentions gomcp/CDP or asks to fetch a URL into Markdown.

## Preconditions
- Preferred tool path: `gomcp` from PATH.
- Default CDP: `ws://127.0.0.1:9222`.
- If gomcp is missing or the CDP endpoint is not reachable, ask the user to fix it before running commands. Do not silently fall back to built-in Web Search or any other search/fetch tools.

## Core commands

- `gomcp duckduckgo <query>` - DuckDuckGo search (recommended, reliable in container environments)
- `gomcp fetch <url>` - Fetch URL and convert to Markdown
- `gomcp warm-tabs` - Warm up browser tabs before batch fetches

## CDP connection recovery

If you see error `dial tcp 127.0.0.1:9222 (default cdp) connection refused`:
1. Run `gomcpman update-browser` to refresh the default CDP instance (or `gomcpman reset-browser` to force recreate)
2. Retry the failed command

## gomcpman management commands

Use `gomcpman` script to manage the browser container:

- `gomcpman update-browser` - Pull latest image (recreate only when updated)
- `gomcpman reset-browser` - Force recreate browser container

## References

- Use `references/gomcp-cli.md` for command/option lookup
- Use `references/gomcpman-help.md` for detailed management command reference

## Behavior

- Use gomcp for all search/fetch tasks when this skill is enabled. Do not call built-in Web Search or other web tools unless gomcp is unavailable and the user explicitly approves a fallback.
- Always use DuckDuckGo for web search (Google search is not supported in container environments).
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

**Search:**
```bash
gomcp duckduckgo "golang tutorial"
gomcp duckduckgo "python best practices"
```

**Fetch:**
```bash
gomcp fetch https://example.com
```

**Batch fetch (use warm-tabs first):**
```bash
gomcp warm-tabs
gomcp fetch https://site1.com
gomcp fetch https://site2.com
```

**CDP troubleshooting:**
```bash
gomcpman update-browser  # Fix connection refused errors
gomcpman reset-browser   # Force recreate browser container
gomcpman status          # Check service health
```
