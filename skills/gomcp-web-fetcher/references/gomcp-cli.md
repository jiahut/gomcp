# gomcp Reference

## Overview
gomcp is a CDP-based CLI for web search and fetch. It can search and fetch pages and convert them to Markdown.

## Commands
| Command | Description |
| --- | --- |
| duckduckgo <query> | DuckDuckGo search (recommended, reliable in container environments) |
| fetch <url> | Fetch URL and convert to Markdown |
| warm-tabs | Warm up browser tabs |

## Options
- -cdp <string> : CDP address (default ws://127.0.0.1:9222)
- -verbose : enable debug logs

## Examples
- gomcp fetch https://example.com
- gomcp duckduckgo "golang tutorial"
