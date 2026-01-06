# gomcp 快速使用指南

## 概述
基于 CDP 的网页操作 CLI，用于搜索、抓取网页转 Markdown。

## 安装路径
`/Users/zhijia.zhang/go/bin/gomcp`

---

## 命令

| 命令 | 功能 |
|------|------|
| `google <query>` | Google 搜索 |
| `duckduckgo <query>` | DuckDuckGo 搜索 |
| `fetch <url>` | 抓取 URL → Markdown |
| `warm-tabs` | 预热浏览器标签 |

## 选项
```
-cdp string   CDP 地址 (默认 ws://127.0.0.1:9222)
-verbose      启用调试日志
```

## 示例
```bash
# 抓取网页
gomcp fetch https://example.com

# 搜索
gomcp google "golang tutorial"

```
