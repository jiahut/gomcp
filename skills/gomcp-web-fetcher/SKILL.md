---
name: gomcp-web-fetcher
description: "Web search and URL fetch to Markdown via gomcp (CDP-based). Use when the user asks to search the web, fetch a URL, or convert a page to Markdown, or mentions gomcp/CDP."
---

# gomcp-web-fetcher Skill

Use the gomcp CLI to perform web search and fetch web pages to Markdown via CDP.

## Preconditions
- Tool path: D:\go\bin\gomcp.exe
- Default CDP: ws://127.0.0.1:9222
- If gomcp is missing or the CDP endpoint is not reachable, ask the user to fix it before running commands.

## Core commands
- google <query>
- duckduckgo <query>
- fetch <url>
- warm-tabs

## Behavior
- Prefer google for broad search unless the user requests DuckDuckGo.
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
- Search:
  D:\go\bin\gomcp.exe google "golang tutorial"
- Fetch:
  D:\go\bin\gomcp.exe fetch https://example.com
- Use a custom CDP:
  D:\go\bin\gomcp.exe -cdp ws://127.0.0.1:9222 fetch https://example.com
