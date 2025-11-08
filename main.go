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
		apiaddr = flags.String("api-addr", env("MCP_API_ADDRESS", ApiDefaultAddress), "http api server address")
		cdp     = flags.String("cdp", os.Getenv("MCP_CDP"), "cdp ws to connect. By default gomcp will run the download Lightpanda browser.")
	)

	// usage func declaration.
	exec := args[0]
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s sse|stdio|download|cleanup|search|fetch [args]\n", exec)
		fmt.Fprintf(stderr, "Demo MCP server.\n")
		fmt.Fprintf(stderr, "\nCommands:\n")
		fmt.Fprintf(stderr, "\tstdio\t\tstarts the stdio server\n")
		fmt.Fprintf(stderr, "\tsse\t\tstarts the HTTP SSE MCP server\n")
		fmt.Fprintf(stderr, "\tdownload\tinstalls or updates the Lightpanda browser\n")
		fmt.Fprintf(stderr, "\tcleanup\tremoves the Lightpanda browser\n")
		fmt.Fprintf(stderr, "\tsearch\t\tweb search and return results\n")
		fmt.Fprintf(stderr, "\tfetch\t\tfetch URL and return markdown content\n")
		fmt.Fprintf(stderr, "\nCommand line options:\n")
		flags.PrintDefaults()
		fmt.Fprintf(stderr, "\nEnvironment vars:\n")
		fmt.Fprintf(stderr, "\tMCP_API_ADDRESS\t\tdefault %s\n", ApiDefaultAddress)
		fmt.Fprintf(stderr, "\tMCP_CDP\n")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	args = flags.Args()
	if len(args) == 0 {
		flags.Usage()
		return errors.New("bad arguments")
	}

	// For commands that don't require browser and take no extra args
	noBrowserCmds := map[string]bool{"cleanup": true, "download": true}
	if noBrowserCmds[args[0]] && len(args) != 1 {
		flags.Usage()
		return errors.New("bad arguments")
	}

	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	// commands w/o browser.
	switch args[0] {
	case "cleanup":
		return cleanup(ctx)
	case "download":
		return download(ctx)
	}

	// commands with browser.
	cdpws := "ws://127.0.0.1:9222"
	if *cdp == "" {
		// Start the local browser.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		browser, err := newbrowser(ctx)
		if err != nil {
			if errors.Is(err, ErrNoBrowser) {
				return errors.New("browser not found. Please run gomcp download first.")
			}
			return fmt.Errorf("new browser: %w", err)
		}

		// Ensure we wait until the browser stops.
		done := make(chan struct{})
		defer func() {
			// wait until the browser stops.
			<-done
		}()

		// Start the browser process.
		go func() {
			if err := browser.Run(); err != nil {
				slog.Error("run browser", slog.Any("err", err))
			}
			// The browser is ended, notify to stop waiting.
			close(done)
		}()

		// Ensure the context is cancelled before waiting the browser end.
		// It will stops the process.
		defer cancel()
	} else {
		cdpws = *cdp
	}

	cdpctx, cancel := chromedp.NewRemoteAllocator(ctx,
		cdpws, chromedp.NoModifyURL,
	)
	defer cancel()

	mcpsrv := NewMCPServer("lightpanda go mcp", "1.0.0", cdpctx)

	switch args[0] {
	case "stdio":
		return runstd(ctx, stdin, stdout, mcpsrv)
	case "sse":
		return runapi(ctx, *apiaddr, mcpsrv)
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

// runSearch performs a web search and returns the results
func runSearch(ctx context.Context, args []string, mcpsrv *MCPServer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("search query is required\nUsage: gomcp search <query>")
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
		return errors.New("URL is required\nUsage: gomcp fetch <url>")
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
