package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/gofrs/flock"
)

// targetStore keeps a pool of reusable browser tab (target) IDs so every
// gomcp process can grab an idle tab, use it in isolation, then return it for
// future callers. This lets concurrent commands run without contending for the
// same page while still avoiding new browser processes.
type targetStore struct {
	mu         sync.Mutex
	path       string
	lock       *flock.Flock
	lastGC     time.Time
	gcInterval time.Duration
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
		path:       statePath,
		lock:       flock.New(lockPath),
		gcInterval: time.Minute,
	}, nil
}

func (s *targetStore) IdleCount() (int, error) {
	if s == nil {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lock.Lock(); err != nil {
		return 0, fmt.Errorf("acquire tab lock: %w", err)
	}
	defer s.lock.Unlock()
	state, err := s.read()
	if err != nil {
		return 0, err
	}
	return len(state.Idle), nil
}

func (s *targetStore) Warm(ctx context.Context, base context.Context, desired int) (int, error) {
	if s == nil {
		return 0, errors.New("tab cache unavailable")
	}
	if base == nil {
		return 0, errors.New("cdp context unavailable")
	}
	if desired <= 0 {
		return 0, nil
	}
	idle, err := s.IdleCount()
	if err != nil {
		return 0, err
	}
	missing := desired - idle
	if missing <= 0 {
		return 0, nil
	}
	created := 0
	for i := 0; i < missing; i++ {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return created, ctx.Err()
			default:
			}
		}
		targetID, err := launchReusableTab(base)
		if err != nil {
			return created, fmt.Errorf("warm tab %d: %w", i+1, err)
		}
		if err := s.Checkin(targetID); err != nil {
			return created, fmt.Errorf("cache warm tab: %w", err)
		}
		created++
	}
	return created, nil
}

func (s *targetStore) GC(ctx context.Context, base context.Context, force bool) (int, bool, error) {
	if s == nil || base == nil {
		return 0, false, nil
	}
	s.mu.Lock()
	if s.gcInterval <= 0 {
		s.gcInterval = time.Minute
	}
	if !force && !s.lastGC.IsZero() && time.Since(s.lastGC) < s.gcInterval {
		s.mu.Unlock()
		return 0, false, nil
	}
	s.lastGC = time.Now()
	if err := s.lock.Lock(); err != nil {
		s.mu.Unlock()
		return 0, false, fmt.Errorf("acquire tab lock: %w", err)
	}
	defer func() {
		s.lock.Unlock()
		s.mu.Unlock()
	}()
	state, err := s.read()
	if err != nil {
		return 0, true, err
	}
	removed := 0
	valid := make([]string, 0, len(state.Idle))
	for _, id := range state.Idle {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return removed, true, ctx.Err()
			default:
			}
		}
		tabCtx, cancel := chromedp.NewContext(base, chromedp.WithTargetID(target.ID(id)))
		if err := chromedp.Run(tabCtx); err != nil {
			removed++
			cancel()
			slog.Debug("tab gc drop target", slog.String("target", id), slog.Any("err", err))
			continue
		}
		detachTargetSession(tabCtx)
		cancel()
		valid = append(valid, id)
	}
	if removed == 0 {
		return 0, true, nil
	}
	state.Idle = valid
	state.compact()
	if err := s.write(state); err != nil {
		return removed, true, err
	}
	return removed, true, nil
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

func launchReusableTab(base context.Context) (string, error) {
	ctx, cancel := chromedp.NewContext(base)
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return "", fmt.Errorf("new tab: %w", err)
	}
	chromedpCtx := chromedp.FromContext(ctx)
	if chromedpCtx == nil || chromedpCtx.Target == nil {
		detachTargetSession(ctx)
		cancel()
		return "", errors.New("missing target metadata")
	}
	id := chromedpCtx.Target.TargetID
	if id == "" {
		detachTargetSession(ctx)
		cancel()
		return "", errors.New("empty target id")
	}
	detachTargetSession(ctx)
	cancel()
	return string(id), nil
}
