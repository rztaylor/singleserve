package singleserve

import (
	"context"
	"fmt"
	"time"
)

// LifetimeMode controls whether browser absence can stop the server.
type LifetimeMode uint8

const (
	// LifetimeExplicit stops only on parent cancellation, owner shutdown, or an
	// authenticated browser shutdown request.
	LifetimeExplicit LifetimeMode = iota
	// LifetimeBrowserBound also stops when first contact never arrives or every
	// contacted browser tab disconnects.
	LifetimeBrowserBound
)

// LifetimePolicy configures browser-presence shutdown behavior.
type LifetimePolicy struct {
	Mode                LifetimeMode
	FirstContactTimeout time.Duration
	HeartbeatTimeout    time.Duration
	DisconnectGrace     time.Duration
	CheckInterval       time.Duration
}

// ExplicitLifetime returns the safe zero-value lifetime policy.
func ExplicitLifetime() LifetimePolicy {
	return LifetimePolicy{Mode: LifetimeExplicit}
}

// BrowserBoundLifetime returns the recommended browser-owned policy.
func BrowserBoundLifetime() LifetimePolicy {
	return LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: 2 * time.Minute,
		HeartbeatTimeout:    15 * time.Second,
		DisconnectGrace:     5 * time.Second,
		CheckInterval:       time.Second,
	}
}

func (p LifetimePolicy) validate() error {
	if p.FirstContactTimeout < 0 || p.HeartbeatTimeout < 0 || p.DisconnectGrace < 0 || p.CheckInterval < 0 {
		return fmt.Errorf("singleserve: lifetime durations cannot be negative")
	}
	switch p.Mode {
	case LifetimeExplicit:
		return nil
	case LifetimeBrowserBound:
		if p.HeartbeatTimeout <= 0 {
			return fmt.Errorf("singleserve: browser-bound heartbeat timeout must be positive")
		}
		if p.CheckInterval <= 0 {
			return fmt.Errorf("singleserve: browser-bound check interval must be positive")
		}
		return nil
	default:
		return fmt.Errorf("singleserve: unknown lifetime mode %d", p.Mode)
	}
}

func (s *Server) watchLifetime(ctx context.Context) {
	ticker := s.clock.NewTicker(s.lifetime.CheckInterval)
	defer ticker.Stop()
	var absentSince time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C():
			s.tabs.expire(now, s.lifetime.HeartbeatTimeout)
			hadContact, connected, _ := s.tabs.state(now)
			if !hadContact {
				if s.lifetime.FirstContactTimeout > 0 && !now.Before(s.startedAt.Add(s.lifetime.FirstContactTimeout)) {
					if s.guardedStop(ctx, ShutdownFirstContactTimeout) {
						return
					}
				}
				continue
			}
			if connected > 0 {
				absentSince = time.Time{}
				continue
			}
			if absentSince.IsZero() {
				absentSince = now
			}
			if now.Before(absentSince.Add(s.lifetime.DisconnectGrace)) {
				continue
			}
			if s.guardedStop(ctx, ShutdownBrowserDisconnected) {
				return
			}
		}
	}
}

func (s *Server) guardedStop(ctx context.Context, reason ShutdownReason) bool {
	if err := s.checkGuard(ctx, reason); err != nil {
		return false
	}
	return s.beginStop(reason)
}
