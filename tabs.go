package singleserve

import (
	"encoding/base64"
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	defaultTabRetention  = 5 * time.Minute
	defaultMaxTabRecords = 256
)

var (
	errInvalidTabID = errors.New("singleserve: invalid tab id")
	// ErrTooManyTabs reports that the bounded tab registry has no reclaimable entries.
	ErrTooManyTabs = errors.New("singleserve: too many connected tabs")
)

// TabState is the current server-side state of one browser tab instance.
type TabState string

const (
	TabConnected    TabState = "connected"
	TabDisconnected TabState = "disconnected"
	TabExpired      TabState = "expired"
)

// TabSnapshot is an immutable copy of tracked browser presence.
type TabSnapshot struct {
	ID             string
	State          TabState
	ConnectedAt    time.Time
	LastSeenAt     time.Time
	DisconnectedAt time.Time
}

type tabRegistry struct {
	mu         sync.Mutex
	records    map[string]TabSnapshot
	hadContact bool
	retention  time.Duration
	maxRecords int
}

func newTabRegistry() *tabRegistry {
	return &tabRegistry{
		records:    make(map[string]TabSnapshot),
		retention:  defaultTabRetention,
		maxRecords: defaultMaxTabRecords,
	}
}

func validateTabID(id string) error {
	if len(id) < 22 || len(id) > 128 {
		return errInvalidTabID
	}
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(decoded) < 16 {
		return errInvalidTabID
	}
	return nil
}

func (r *tabRegistry) heartbeat(id string, now time.Time) error {
	if err := validateTabID(id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pruneLocked(now)
	record, exists := r.records[id]
	if !exists && len(r.records) >= r.maxRecords {
		return ErrTooManyTabs
	}
	if !exists {
		record = TabSnapshot{ID: id, ConnectedAt: now}
	}
	record.State = TabConnected
	record.LastSeenAt = now
	record.DisconnectedAt = time.Time{}
	r.records[id] = record
	r.hadContact = true
	return nil
}

func (r *tabRegistry) disconnect(id string, now time.Time) error {
	if err := validateTabID(id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.records[id]
	if !exists || record.State != TabConnected {
		return nil
	}
	record.State = TabDisconnected
	record.DisconnectedAt = now
	r.records[id] = record
	return nil
}

func (r *tabRegistry) expire(now time.Time, timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, record := range r.records {
		if record.State != TabConnected || now.Before(record.LastSeenAt.Add(timeout)) {
			continue
		}
		record.State = TabExpired
		record.DisconnectedAt = now
		r.records[id] = record
	}
	r.pruneLocked(now)
}

func (r *tabRegistry) state(now time.Time) (bool, int, []TabSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	connected := 0
	snapshots := make([]TabSnapshot, 0, len(r.records))
	for _, record := range r.records {
		if record.State == TabConnected {
			connected++
		}
		snapshots = append(snapshots, record)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ConnectedAt.Equal(snapshots[j].ConnectedAt) {
			return snapshots[i].ID < snapshots[j].ID
		}
		return snapshots[i].ConnectedAt.Before(snapshots[j].ConnectedAt)
	})
	return r.hadContact, connected, snapshots
}

func (r *tabRegistry) snapshots(now time.Time) []TabSnapshot {
	_, _, snapshots := r.state(now)
	return snapshots
}

func (r *tabRegistry) pruneLocked(now time.Time) {
	for id, record := range r.records {
		if record.State == TabConnected || record.DisconnectedAt.IsZero() {
			continue
		}
		if !now.Before(record.DisconnectedAt.Add(r.retention)) {
			delete(r.records, id)
		}
	}
	if len(r.records) < r.maxRecords {
		return
	}

	type terminalRecord struct {
		id   string
		when time.Time
	}
	var terminal []terminalRecord
	for id, record := range r.records {
		if record.State != TabConnected {
			terminal = append(terminal, terminalRecord{id: id, when: record.DisconnectedAt})
		}
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].when.Before(terminal[j].when) })
	for len(r.records) >= r.maxRecords && len(terminal) > 0 {
		delete(r.records, terminal[0].id)
		terminal = terminal[1:]
	}
}
