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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/gofrs/flock"
)

const (
	defaultMinIdleTabs   = 1
	defaultBurstIdleTabs = 2
	defaultMaxIdleTabs   = 3
	defaultIdleTabTTL    = 10 * time.Minute
)

type tabPoolConfig struct {
	MinIdle   int
	BurstIdle int
	MaxIdle   int
	IdleTTL   time.Duration
}

// targetStore keeps a pool of reusable browser tab (target) IDs so every
// gomcp process can grab an idle tab, use it in isolation, then return it for
// future callers. This lets concurrent commands run without contending for the
// same page while still avoiding new browser processes.
type targetStore struct {
	mu            sync.Mutex
	path          string
	lock          *flock.Flock
	lastGC        time.Time
	gcInterval    time.Duration
	cfg           tabPoolConfig
	ensurePending int
	ensureRunning bool
}

type idleTarget struct {
	ID         string    `json:"id"`
	ParkedAt   time.Time `json:"parkedAt,omitempty"`
	LastUsedAt time.Time `json:"lastUsedAt,omitempty"`
}

type targetState struct {
	Idle []idleTarget `json:"idle"`
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
		cfg:        loadTabPoolConfig(),
	}, nil
}

func loadTabPoolConfig() tabPoolConfig {
	cfg := tabPoolConfig{
		MinIdle:   defaultMinIdleTabs,
		BurstIdle: defaultBurstIdleTabs,
		MaxIdle:   defaultMaxIdleTabs,
		IdleTTL:   defaultIdleTabTTL,
	}

	cfg.MinIdle = envInt("GOMCP_TAB_MIN_IDLE", cfg.MinIdle)
	cfg.BurstIdle = envInt("GOMCP_TAB_BURST_IDLE", cfg.BurstIdle)
	cfg.MaxIdle = envInt("GOMCP_TAB_MAX_IDLE", cfg.MaxIdle)
	if raw, ok := os.LookupEnv("GOMCP_TAB_IDLE_TTL"); ok {
		if ttl, err := time.ParseDuration(strings.TrimSpace(raw)); err == nil && ttl > 0 {
			cfg.IdleTTL = ttl
		}
	}

	if cfg.MinIdle < 0 {
		cfg.MinIdle = 0
	}
	if cfg.BurstIdle < cfg.MinIdle {
		cfg.BurstIdle = cfg.MinIdle
	}
	if cfg.MaxIdle < cfg.BurstIdle {
		cfg.MaxIdle = cfg.BurstIdle
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = defaultIdleTabTTL
	}

	return cfg
}

func envInt(key string, dflt int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return dflt
	}
	val, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return dflt
	}
	return val
}

func (s *targetStore) MinIdle() int {
	if s == nil {
		return 0
	}
	return s.cfg.MinIdle
}

func (s *targetStore) BurstIdle() int {
	if s == nil {
		return 0
	}
	return s.cfg.BurstIdle
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

func (s *targetStore) EnsureIdle(ctx context.Context, base context.Context, desired int) (int, error) {
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
		now := time.Now()
		if err := s.Checkin(newIdleTarget(targetID, now)); err != nil {
			return created, fmt.Errorf("cache warm tab: %w", err)
		}
		created++
	}
	return created, nil
}

func (s *targetStore) ScheduleEnsureIdle(base context.Context, desired int) {
	if s == nil || base == nil || desired <= 0 {
		return
	}

	s.mu.Lock()
	if desired > s.ensurePending {
		s.ensurePending = desired
	}
	if s.ensureRunning {
		s.mu.Unlock()
		return
	}
	s.ensureRunning = true
	s.mu.Unlock()

	go s.runEnsureLoop(base)
}

func (s *targetStore) runEnsureLoop(base context.Context) {
	for {
		s.mu.Lock()
		desired := s.ensurePending
		s.ensurePending = 0
		s.mu.Unlock()

		if desired > 0 {
			ctx, cancel := context.WithTimeout(base, 10*time.Second)
			_, err := s.EnsureIdle(ctx, base, desired)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Debug("ensure idle tabs", slog.Int("desired", desired), slog.Any("err", err))
			}
		}

		s.mu.Lock()
		if s.ensurePending == 0 {
			s.ensureRunning = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
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

	now := time.Now()
	keep, drop := state.gcCandidates(now, s.cfg)
	removed := 0
	valid := make([]idleTarget, 0, len(keep))
	for _, tab := range keep {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return removed, true, ctx.Err()
			default:
			}
		}

		tabCtx, cancel := chromedp.NewContext(base, chromedp.WithTargetID(target.ID(tab.ID)))
		if err := chromedp.Run(tabCtx); err != nil {
			removed++
			cancel()
			slog.Debug("tab gc drop invalid target", slog.String("target", tab.ID), slog.Any("err", err))
			continue
		}
		detachTargetSession(tabCtx)
		cancel()
		valid = append(valid, tab)
	}

	for _, tab := range drop {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return removed, true, ctx.Err()
			default:
			}
		}

		if err := closeTarget(base, target.ID(tab.ID)); err != nil {
			slog.Debug("tab gc close target", slog.String("target", tab.ID), slog.Any("err", err))
		}
		removed++
	}

	next := targetState{Idle: valid}
	next.compact()
	if removed == 0 && idleTargetsEqual(state.Idle, next.Idle) {
		return 0, true, nil
	}
	if err := s.write(next); err != nil {
		return removed, true, err
	}

	return removed, true, nil
}

func (s *targetStore) Checkout() (idleTarget, int, error) {
	if s == nil {
		return idleTarget{}, 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.lock.Lock(); err != nil {
		return idleTarget{}, 0, fmt.Errorf("acquire tab lock: %w", err)
	}
	defer s.lock.Unlock()

	state, err := s.read()
	if err != nil {
		return idleTarget{}, 0, err
	}
	if len(state.Idle) == 0 {
		return idleTarget{}, 0, nil
	}

	tab := state.Idle[0]
	state.Idle = append([]idleTarget{}, state.Idle[1:]...)
	if err := s.write(state); err != nil {
		return idleTarget{}, 0, err
	}

	return tab, len(state.Idle), nil
}

func (s *targetStore) Checkin(tab idleTarget) error {
	if s == nil || strings.TrimSpace(tab.ID) == "" {
		return nil
	}

	tab = tab.normalized()

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

	state.Idle = append(state.Idle, tab)
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
	return replaceFile(tmp, s.path)
}

func (state *targetState) compact() {
	if len(state.Idle) == 0 {
		return
	}

	merged := make(map[string]idleTarget, len(state.Idle))
	for _, tab := range state.Idle {
		tab = tab.normalized()
		if tab.ID == "" {
			continue
		}
		if existing, ok := merged[tab.ID]; ok {
			merged[tab.ID] = mergeIdleTarget(existing, tab)
			continue
		}
		merged[tab.ID] = tab
	}

	idle := make([]idleTarget, 0, len(merged))
	for _, tab := range merged {
		idle = append(idle, tab)
	}
	sort.Slice(idle, func(i, j int) bool {
		ti := idle[i].sortTime()
		tj := idle[j].sortTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return idle[i].ID < idle[j].ID
	})

	state.Idle = idle
}

func (state targetState) gcCandidates(now time.Time, cfg tabPoolConfig) ([]idleTarget, []idleTarget) {
	state.compact()

	keep := make([]idleTarget, 0, len(state.Idle))
	drop := make([]idleTarget, 0)
	for _, tab := range state.Idle {
		if cfg.IdleTTL > 0 {
			parkedAt := tab.sortTime()
			if !parkedAt.IsZero() && now.Sub(parkedAt) > cfg.IdleTTL {
				drop = append(drop, tab)
				continue
			}
		}
		keep = append(keep, tab)
	}

	if cfg.MaxIdle > 0 && len(keep) > cfg.MaxIdle {
		drop = append(drop, keep[cfg.MaxIdle:]...)
		keep = keep[:cfg.MaxIdle]
	}

	return keep, drop
}

func idleTargetsEqual(left, right []idleTarget) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].ID != right[i].ID || !left[i].ParkedAt.Equal(right[i].ParkedAt) || !left[i].LastUsedAt.Equal(right[i].LastUsedAt) {
			return false
		}
	}
	return true
}

func newIdleTarget(id string, now time.Time) idleTarget {
	return idleTarget{
		ID:         strings.TrimSpace(id),
		ParkedAt:   now.UTC(),
		LastUsedAt: now.UTC(),
	}
}

func mergeIdleTarget(left, right idleTarget) idleTarget {
	out := left.normalized()
	right = right.normalized()
	if out.ParkedAt.Before(right.ParkedAt) {
		out.ParkedAt = right.ParkedAt
	}
	if out.LastUsedAt.Before(right.LastUsedAt) {
		out.LastUsedAt = right.LastUsedAt
	}
	return out
}

func (t idleTarget) normalized() idleTarget {
	t.ID = strings.TrimSpace(t.ID)
	if t.ParkedAt.IsZero() && !t.LastUsedAt.IsZero() {
		t.ParkedAt = t.LastUsedAt
	}
	if t.LastUsedAt.IsZero() && !t.ParkedAt.IsZero() {
		t.LastUsedAt = t.ParkedAt
	}
	if !t.ParkedAt.IsZero() {
		t.ParkedAt = t.ParkedAt.UTC()
	}
	if !t.LastUsedAt.IsZero() {
		t.LastUsedAt = t.LastUsedAt.UTC()
	}
	return t
}

func (t idleTarget) sortTime() time.Time {
	if !t.ParkedAt.IsZero() {
		return t.ParkedAt
	}
	return t.LastUsedAt
}

func (t *idleTarget) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		*t = idleTarget{ID: strings.TrimSpace(id)}
		return nil
	}

	type alias idleTarget
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*t = idleTarget(raw).normalized()
	return nil
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
	if id, err := createBackgroundTarget(base); err == nil && id != "" {
		return string(id), nil
	} else if err != nil {
		slog.Debug("create background tab failed, falling back", slog.Any("err", err))
	}

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
