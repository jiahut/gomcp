package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gofrs/flock"
)

// targetStore keeps a pool of reusable browser tab (target) IDs so every
// gomcp process can grab an idle tab, use it in isolation, then return it for
// future callers. This lets concurrent commands run without contending for the
// same page while still avoiding new browser processes.
type targetStore struct {
	mu   sync.Mutex
	path string
	lock *flock.Flock
}

type targetState struct {
	Idle []string `json:"idle"`
}

func newTargetStore(key string) (*targetStore, error) {
	dir, err := configdir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o775); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("ensure config dir: %w", err)
	}

	base := sanitizeKey(key)
	statePath := filepath.Join(dir, fmt.Sprintf("tabs-%s.json", base))
	lockPath := statePath + ".lock"

	return &targetStore{
		path: statePath,
		lock: flock.New(lockPath),
	}, nil
}

func (s *targetStore) Checkout() (string, error) {
	if s == nil {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.lock.Lock(); err != nil {
		return "", fmt.Errorf("acquire tab lock: %w", err)
	}
	defer s.lock.Unlock()

	state, err := s.read()
	if err != nil {
		return "", err
	}
	if len(state.Idle) == 0 {
		return "", nil
	}

	id := state.Idle[0]
	state.Idle = append([]string{}, state.Idle[1:]...)
	if err := s.write(state); err != nil {
		return "", err
	}

	return id, nil
}

func (s *targetStore) Checkin(id string) error {
	if s == nil || id == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.lock.Lock(); err != nil {
		return fmt.Errorf("acquire tab lock: %w", err)
	}
	defer s.lock.Unlock()

	state, err := s.read()
	if err != nil {
		return err
	}

	for _, existing := range state.Idle {
		if existing == id {
			return nil
		}
	}
	state.Idle = append(state.Idle, id)
	state.compact()

	return s.write(state)
}

func (s *targetStore) Clear() error {
	if s == nil {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove tab state: %w", err)
	}
	return nil
}

func (s *targetStore) read() (targetState, error) {
	var state targetState
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, fmt.Errorf("read tab state: %w", err)
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return targetState{}, fmt.Errorf("decode tab state: %w", err)
	}
	state.compact()
	return state, nil
}

func (s *targetStore) write(state targetState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode tab state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tab state: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func (state *targetState) compact() {
	if len(state.Idle) == 0 {
		return
	}
	set := make(map[string]struct{}, len(state.Idle))
	idle := make([]string, 0, len(state.Idle))
	for _, id := range state.Idle {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		idle = append(idle, id)
	}
	sort.Strings(idle)
	state.Idle = idle
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "default"
	}
	key = strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_").Replace(key)
	if len(key) > 40 {
		return key[:40]
	}
	return key
}
