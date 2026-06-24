package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestTargetStoreWriteOverwritesExistingState(t *testing.T) {
	store := &targetStore{path: filepath.Join(t.TempDir(), "tabs.json")}
	first := targetState{Idle: []idleTarget{newIdleTarget("first", time.Now())}}
	second := targetState{Idle: []idleTarget{newIdleTarget("second", time.Now())}}

	if err := store.write(first); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := store.write(second); err != nil {
		t.Fatalf("overwrite state: %v", err)
	}

	got, err := store.read()
	if err != nil {
		t.Fatalf("read overwritten state: %v", err)
	}
	if len(got.Idle) != 1 || got.Idle[0].ID != "second" {
		t.Fatalf("expected overwritten state, got %#v", got.Idle)
	}
}

func TestTargetStateSupportsLegacyStringEntries(t *testing.T) {
	var state targetState
	if err := json.Unmarshal([]byte(`{"idle":["  abc  "]}`), &state); err != nil {
		t.Fatalf("unmarshal legacy state: %v", err)
	}

	if len(state.Idle) != 1 {
		t.Fatalf("expected 1 idle target, got %d", len(state.Idle))
	}
	if state.Idle[0].ID != "abc" {
		t.Fatalf("expected trimmed id, got %q", state.Idle[0].ID)
	}
}

func TestTargetStateCompactKeepsNewestMetadata(t *testing.T) {
	now := time.Date(2026, 3, 21, 15, 0, 0, 0, time.UTC)
	state := targetState{
		Idle: []idleTarget{
			{ID: "b", ParkedAt: now.Add(-2 * time.Minute)},
			{ID: "a", ParkedAt: now.Add(-3 * time.Minute)},
			{ID: "a", ParkedAt: now.Add(-1 * time.Minute), LastUsedAt: now.Add(-1 * time.Minute)},
		},
	}

	state.compact()

	if len(state.Idle) != 2 {
		t.Fatalf("expected 2 unique idle targets, got %d", len(state.Idle))
	}
	if state.Idle[0].ID != "a" {
		t.Fatalf("expected newest target first, got %q", state.Idle[0].ID)
	}
	if !state.Idle[0].ParkedAt.Equal(now.Add(-1 * time.Minute)) {
		t.Fatalf("expected latest parkedAt to win, got %s", state.Idle[0].ParkedAt)
	}
}

func TestTargetStateGCCandidatesDropExpiredAndExcess(t *testing.T) {
	now := time.Date(2026, 3, 21, 15, 0, 0, 0, time.UTC)
	state := targetState{
		Idle: []idleTarget{
			newIdleTarget("fresh-1", now.Add(-1*time.Minute)),
			newIdleTarget("fresh-2", now.Add(-2*time.Minute)),
			newIdleTarget("fresh-3", now.Add(-3*time.Minute)),
			newIdleTarget("expired", now.Add(-20*time.Minute)),
		},
	}

	keep, drop := state.gcCandidates(now, tabPoolConfig{
		MinIdle:   1,
		BurstIdle: 2,
		MaxIdle:   2,
		IdleTTL:   5 * time.Minute,
	})

	if len(keep) != 2 {
		t.Fatalf("expected 2 kept targets, got %d", len(keep))
	}
	if keep[0].ID != "fresh-1" || keep[1].ID != "fresh-2" {
		t.Fatalf("unexpected keep set: %#v", keep)
	}
	if len(drop) != 2 {
		t.Fatalf("expected 2 dropped targets, got %d", len(drop))
	}
	if drop[0].ID != "expired" && drop[1].ID != "expired" {
		t.Fatalf("expected expired target to be dropped: %#v", drop)
	}
}
