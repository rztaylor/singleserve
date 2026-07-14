package singleserve

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func testTabID(seed byte) string {
	value := make([]byte, 16)
	for i := range value {
		value[i] = seed
	}
	return base64.RawURLEncoding.EncodeToString(value)
}

func TestValidateTabID(t *testing.T) {
	valid := testTabID(1)
	if err := validateTabID(valid); err != nil {
		t.Fatalf("valid tab: %v", err)
	}
	for _, invalid := range []string{"", "short", valid + "=", "not+url/safe________"} {
		if !errors.Is(validateTabID(invalid), errInvalidTabID) {
			t.Fatalf("tab %q unexpectedly valid", invalid)
		}
	}
}

func TestTabRegistryTracksIndependentStates(t *testing.T) {
	registry := newTabRegistry()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	first := testTabID(1)
	second := testTabID(2)
	if err := registry.heartbeat(first, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.heartbeat(second, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := registry.disconnect(first, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}

	hadContact, connected, snapshots := registry.state(now.Add(2 * time.Second))
	if !hadContact || connected != 1 || len(snapshots) != 2 {
		t.Fatalf("state contact=%v connected=%d snapshots=%#v", hadContact, connected, snapshots)
	}
	if snapshots[0].ID != first || snapshots[0].State != TabDisconnected || snapshots[1].State != TabConnected {
		t.Fatalf("snapshots = %#v", snapshots)
	}

	registry.expire(now.Add(20*time.Second), 15*time.Second)
	_, connected, snapshots = registry.state(now.Add(20 * time.Second))
	if connected != 0 || snapshots[1].State != TabExpired {
		t.Fatalf("expired snapshots = %#v", snapshots)
	}
}

func TestTabRegistryReconnectAndUnknownDisconnect(t *testing.T) {
	registry := newTabRegistry()
	now := time.Now()
	id := testTabID(3)
	if err := registry.disconnect(id, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.heartbeat(id, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.disconnect(id, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := registry.heartbeat(id, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	snapshots := registry.snapshots(now.Add(2 * time.Second))
	if len(snapshots) != 1 || snapshots[0].State != TabConnected || !snapshots[0].DisconnectedAt.IsZero() || !snapshots[0].ConnectedAt.Equal(now) {
		t.Fatalf("reconnected snapshot = %#v", snapshots)
	}
}

func TestTabRegistryBoundsAndPrunesRecords(t *testing.T) {
	registry := newTabRegistry()
	registry.maxRecords = 2
	registry.retention = time.Minute
	now := time.Now()
	first := testTabID(4)
	second := testTabID(5)
	third := testTabID(6)
	if err := registry.heartbeat(first, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.heartbeat(second, now); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(registry.heartbeat(third, now), ErrTooManyTabs) {
		t.Fatal("expected connected tab bound")
	}
	if err := registry.disconnect(first, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.heartbeat(third, now); err != nil {
		t.Fatalf("terminal record should be reclaimable: %v", err)
	}
	if got := registry.snapshots(now); len(got) != 2 {
		t.Fatalf("bounded snapshots = %#v", got)
	}
	if err := registry.disconnect(second, now); err != nil {
		t.Fatal(err)
	}
	if err := registry.disconnect(third, now); err != nil {
		t.Fatal(err)
	}
	if got := registry.snapshots(now.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("retained snapshots = %#v", got)
	}
}
