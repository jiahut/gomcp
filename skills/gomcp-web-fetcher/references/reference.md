# gomcp Reference

## Overview
gomcp is a CDP-based CLI for web search and fetch. It connects to a browser via Chrome DevTools Protocol, performs actions, and returns results as Markdown.

## Commands

| Command | Description | Output |
|---------|-------------|--------|
| `google <query>` | Google search | Up to 10 results: title, link, snippet |
| `duckduckgo <query>` | DuckDuckGo search | Up to 10 results: title, link, snippet |
| `fetch <url>` | Fetch URL → Markdown | Page content as Markdown |

## Global Options

| Option | Default | Description |
|--------|---------|-------------|
| `-cdp <addr>` | `ws://127.0.0.1:9222` | CDP WebSocket address |
| `-verbose` | off | Enable debug logs |

## Error Scenarios & Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `gomcp: command not found` | Binary not on PATH | Install gomcp and ensure it is on PATH |
| `websocket: bad handshake` / connection refused | CDP endpoint not running | Start the browser or verify `-cdp` address |
| Timeout with no output | Page takes too long to load | Retry; if persistent, check network or try a simpler URL |
| Empty search results | Query too narrow or page structure changed | Refine query; try the other search engine |
| Garbled/truncated Markdown | Non-UTF8 encoding or very large page | Fetch a cached/simpler version; summarize output |
| `context deadline exceeded` | Browser tab hung | Retry the command; if persistent, restart the browser |

## Edge Cases

- **Redirects**: gomcp follows redirects automatically. The final URL may differ from the input.
- **JavaScript-heavy pages**: Content rendered after JS execution is captured (CDP waits for load).
- **Large pages**: Output can be very long. Summarize for the user and offer full content if needed.
- **Rate limiting**: Google may rate-limit repeated searches. Space out requests or switch to DuckDuckGo.
- **Non-HTML content**: PDFs or binary files will produce minimal or no Markdown output.

## Examples

```bash
# Basic search
gomcp google "golang tutorial"

# DuckDuckGo search
gomcp duckduckgo "rust async programming"

# Fetch a specific page
gomcp fetch https://example.com

# Custom CDP endpoint
gomcp -cdp ws://192.168.1.100:9222 fetch https://example.com

# Debug mode
gomcp -verbose google "test query"
```
