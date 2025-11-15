// Copyright 2025 Lightpanda (Selecy SAS)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	exitOK   = 0
	exitFail = 1
)

// main starts interruptable context and runs the program.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	err := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitFail)
	}

	os.Exit(exitOK)
}

const (
	ApiDefaultAddress = "127.0.0.1:8081"
)

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// declare runtime flag parameters.
	flags := flag.NewFlagSet(args[0], flag.ExitOnError)
	flags.SetOutput(stderr)

	var (
		verbose = flags.Bool("verbose", false, "enable debug log level")
		cdp     = flags.String("cdp", "ws://127.0.0.1:9222", "cdp ws to connect")
	)

	// usage func declaration.
	exec := args[0]
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s search|fetch [args]\n", exec)
		fmt.Fprintf(stderr, "\nCommands:\n")
		fmt.Fprintf(stderr, "\tsearch\t\tweb search and return results\n")
		fmt.Fprintf(stderr, "\tfetch\t\tfetch URL and return markdown content\n")
		fmt.Fprintf(stderr, "\nCommand line options:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	args = flags.Args()
	if len(args) == 0 {
		flags.Usage()
		return errors.New("bad arguments")
	}

	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	endpointStore, err := newCDPEndpointStore()
	if err != nil {
		slog.Warn("cdp endpoint cache disabled", slog.Any("err", err))
	}

	resolvedCDP := *cdp
	key := ""
	if resolved, hostKey, err := resolveCDPEndpoint(ctx, *cdp, endpointStore); err != nil {
		slog.Warn("resolve cdp", slog.Any("err", err))
		key, _ = cdpStoreKey(*cdp)
	} else {
		resolvedCDP = resolved
		key = hostKey
	}

	var tabStore *targetStore
	if key != "" {
		if store, err := newTargetStore(key); err != nil {
			slog.Warn("init tab cache", slog.Any("err", err))
		} else {
			tabStore = store
		}
	}

	// Connect to CDP browser
	cdpctx, cancel := chromedp.NewRemoteAllocator(ctx, resolvedCDP, chromedp.NoModifyURL)
	defer cancel()

	mcpsrv := NewMCPServer("lightpanda go mcp", "1.0.0", cdpctx, tabStore)

	switch args[0] {
	case "search":
		return runSearch(ctx, args[1:], mcpsrv, stderr)
	case "fetch":
		return runFetch(ctx, args[1:], mcpsrv, stderr)
	}

	flags.Usage()
	return errors.New("bad command")
}

// env returns the env value corresponding to the key or the default string.
func env(key, dflt string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return dflt
	}

	return val
}

func cdpStoreKey(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("empty cdp address")
	}
	addr := raw
	if !strings.Contains(addr, "://") {
		addr = "ws://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("parse cdp url: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host in cdp url: %s", raw)
	}
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		u.Host = net.JoinHostPort(u.Host, "9222")
	}
	return u.Host, nil
}

// runSearch performs a web search and returns the results
func runSearch(ctx context.Context, args []string, mcpsrv *MCPServer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(`search query is required

Usage:
  gomcp search <query>

Description:
  Perform a web search using DuckDuckGo and return a list of results
  including titles, links, and snippets.

Example:
  gomcp search "Go programming language"
  gomcp search "人工智能最新进展"
  gomcp search "golang tutorial beginner"`)
	}

	// Join all arguments to form the search query (allows spaces in search terms)
	query := strings.Join(args, " ")

	conn := mcpsrv.NewConn()
	defer conn.Close()

	// Perform search using DuckDuckGo
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	if _, err := conn.Goto(searchURL); err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	slog.Info("Searching for", slog.String("query", query))

	// Wait for results to load
	chromedp.Run(conn.cdpctx, chromedp.Sleep(2*time.Second))

	// Extract search results
	var results []SearchResult
	err := chromedp.Run(conn.cdpctx,
		chromedp.Evaluate(`(() => {
			const results = [];
			// DuckDuckGo HTML version selectors
			const resultNodes = document.querySelectorAll('div.result');

			resultNodes.forEach((node) => {
				const titleNode = node.querySelector('a.result__a');
				const linkNode = node.querySelector('a.result__a');
				const snippetNode = node.querySelector('a.result__snippet');

				if (titleNode && linkNode) {
					const title = titleNode.textContent?.trim() || '';
					const link = linkNode.href || '';
					const snippet = snippetNode ? (snippetNode.textContent?.trim() || '') : '';

					if (title && link) {
						results.push({ title, link, snippet });
					}
				}
			});

			return results.slice(0, 10);
		})()`, &results),
	)

	if err != nil {
		return fmt.Errorf("extract results: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintf(stderr, "No results found for query: %s\n", query)
		return nil
	}

	// Print results
	fmt.Printf("Search results for: %s\n\n", query)
	for i, result := range results {
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.Link)
		if result.Snippet != "" {
			fmt.Printf("   Snippet: %s\n", result.Snippet)
		}
		fmt.Println()
	}

	return nil
}

// runFetch fetches a URL and returns its markdown content
func runFetch(ctx context.Context, args []string, mcpsrv *MCPServer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(`URL is required

Usage:
  gomcp fetch <url>

Description:
  Fetch a webpage from the given URL and return its content in
  Markdown format. This is useful for converting web pages to
  readable text or extracting content for further processing.

Example:
  gomcp fetch "https://go.dev"
  gomcp fetch "https://example.com" > page.md
  gomcp fetch "https://news.ycombinator.com" | grep "Go"`)
	}

	url := args[0]

	conn := mcpsrv.NewConn()
	defer conn.Close()

	slog.Info("Fetching", slog.String("url", url))

	// Navigate to the URL
	if _, err := conn.Goto(url); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Get markdown content
	markdown, err := conn.GetMarkdown()
	if err != nil {
		return fmt.Errorf("convert to markdown: %w", err)
	}

	// Print the markdown
	fmt.Print(markdown)

	return nil
}

// SearchResult represents a search result
type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}
