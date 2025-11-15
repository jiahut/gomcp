package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type cdpEndpointStore struct {
	path string
	mu   sync.Mutex
}

type cdpEndpointState struct {
	Entries map[string]string `json:"entries"`
}

func newCDPEndpointStore() (*cdpEndpointStore, error) {
	dir, err := configdir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o775); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	return &cdpEndpointStore{path: filepath.Join(dir, "cdp-endpoints.json")}, nil
}

func (s *cdpEndpointStore) Get(host string) (string, bool, error) {
	if s == nil {
		return "", false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.read()
	if err != nil {
		return "", false, err
	}
	val, ok := state.Entries[host]
	return val, ok, nil
}

func (s *cdpEndpointStore) Remember(host, wsURL string) error {
	if s == nil || host == "" || wsURL == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.read()
	if err != nil {
		return err
	}
	state.Entries[host] = wsURL
	return s.write(state)
}

func (s *cdpEndpointStore) read() (cdpEndpointState, error) {
	state := cdpEndpointState{Entries: make(map[string]string)}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, err
	}
	if len(b) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return cdpEndpointState{Entries: make(map[string]string)}, err
	}
	if state.Entries == nil {
		state.Entries = make(map[string]string)
	}
	return state, nil
}

func (s *cdpEndpointStore) write(state cdpEndpointState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func resolveCDPEndpoint(ctx context.Context, raw string, store *cdpEndpointStore) (string, string, error) {
	norm := raw
	if norm == "" {
		norm = "ws://127.0.0.1:9222"
	}
	if !strings.Contains(norm, "://") {
		norm = "ws://" + norm
	}
	u, err := url.Parse(norm)
	if err != nil {
		return "", "", fmt.Errorf("parse cdp url: %w", err)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("missing host in cdp url: %s", raw)
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "9222")
		u.Host = host
	}
	if strings.Contains(u.Path, "/devtools/browser/") {
		return u.String(), host, nil
	}
	if store != nil {
		if cached, ok, err := store.Get(host); err == nil && ok {
			return cached, host, nil
		} else if err != nil {
			slog.Warn("read cached cdp endpoint", slog.Any("err", err))
		}
	}
	wsURL, err := fetchWebsocketDebuggerURL(ctx, u.Scheme, host)
	if err != nil {
		return u.String(), host, err
	}
	if store != nil {
		if err := store.Remember(host, wsURL); err != nil {
			slog.Warn("remember cdp endpoint", slog.Any("err", err))
		}
	}
	return wsURL, host, nil
}

func fetchWebsocketDebuggerURL(ctx context.Context, scheme, host string) (string, error) {
	httpScheme := "http"
	if scheme == "https" || scheme == "wss" {
		httpScheme = "https"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	endpoint := fmt.Sprintf("%s://%s/json/version", httpScheme, host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("json/version status %d", resp.StatusCode)
	}
	var payload struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.WS == "" {
		return "", errors.New("empty webSocketDebuggerUrl")
	}
	return payload.WS, nil
}
