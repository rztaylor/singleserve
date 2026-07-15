package singleserve

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifetimePolicyValidation(t *testing.T) {
	if got := ExplicitLifetime(); got.Mode != LifetimeExplicit {
		t.Fatalf("explicit lifetime = %#v", got)
	}
	browser := BrowserBoundLifetime()
	if browser.Mode != LifetimeBrowserBound || browser.FirstContactTimeout != 2*time.Minute || browser.HeartbeatTimeout != 15*time.Second || browser.DisconnectGrace != 5*time.Second || browser.CheckInterval != time.Second {
		t.Fatalf("browser lifetime defaults = %#v", browser)
	}

	tests := []LifetimePolicy{
		{Mode: LifetimeMode(99)},
		{Mode: LifetimeExplicit, CheckInterval: -time.Second},
		{Mode: LifetimeBrowserBound, HeartbeatTimeout: 0, CheckInterval: time.Second},
		{Mode: LifetimeBrowserBound, HeartbeatTimeout: time.Second, CheckInterval: 0},
	}
	for _, policy := range tests {
		if err := policy.validate(); err == nil {
			t.Fatalf("policy %#v unexpectedly valid", policy)
		}
	}
	if err := (LifetimePolicy{Mode: LifetimeBrowserBound, HeartbeatTimeout: time.Second, CheckInterval: time.Second}).validate(); err != nil {
		t.Fatalf("valid custom browser lifetime: %v", err)
	}
}

type manualClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*manualTicker
	created chan struct{}
}

type manualTicker struct {
	mu      sync.Mutex
	channel chan time.Time
	stopped bool
}

func newManualClock(now time.Time) *manualClock {
	return &manualClock{now: now, created: make(chan struct{}, 8)}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) NewTicker(time.Duration) ticker {
	t := &manualTicker{channel: make(chan time.Time, 8)}
	c.mu.Lock()
	c.tickers = append(c.tickers, t)
	c.mu.Unlock()
	c.created <- struct{}{}
	return t
}

func (c *manualClock) advance(now time.Time) {
	c.mu.Lock()
	c.now = now
	tickers := append([]*manualTicker(nil), c.tickers...)
	c.mu.Unlock()
	for _, t := range tickers {
		t.mu.Lock()
		if !t.stopped {
			t.channel <- now
		}
		t.mu.Unlock()
	}
}

func (t *manualTicker) C() <-chan time.Time { return t.channel }

func (t *manualTicker) Stop() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()
}

func TestBrowserBoundFirstContactTimeout(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server := newTestServerWithClock(t, clock, LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: 10 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		DisconnectGrace:     2 * time.Second,
		CheckInterval:       time.Second,
	}, nil)
	launch := startWithTicker(t, server, clock)
	clock.advance(now.Add(10 * time.Second))
	result := waitForResult(t, launch)
	if result.Reason != ShutdownFirstContactTimeout {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestBootstrapRenewalDoesNotResetFirstContactTimeout(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server, err := newServer(Options{
		Handler: http.NotFoundHandler(),
		Lifetime: LifetimePolicy{
			Mode:                LifetimeBrowserBound,
			FirstContactTimeout: 10 * time.Second,
			HeartbeatTimeout:    5 * time.Second,
			DisconnectGrace:     2 * time.Second,
			CheckInterval:       time.Second,
		},
	}, clock, nil, strings.NewReader(strings.Repeat("t", 144)))
	if err != nil {
		t.Fatal(err)
	}
	launch := startWithTicker(t, server, clock)
	clock.advance(now.Add(5 * time.Second))
	if _, err := launch.NewBootstrapURL(); err != nil {
		t.Fatal(err)
	}
	clock.advance(now.Add(10 * time.Second))
	result := waitForResult(t, launch)
	if result.Reason != ShutdownFirstContactTimeout {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestBrowserBoundExpiresIndependentTabsThenStops(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server := newTestServerWithClock(t, clock, LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: time.Minute,
		HeartbeatTimeout:    10 * time.Second,
		DisconnectGrace:     5 * time.Second,
		CheckInterval:       time.Second,
	}, nil)
	launch := startWithTicker(t, server, clock)
	if err := server.tabs.heartbeat(testTabID(1), now); err != nil {
		t.Fatal(err)
	}
	if err := server.tabs.heartbeat(testTabID(2), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	clock.advance(now.Add(10 * time.Second))
	waitFor(t, func() bool {
		tabs := server.Tabs()
		return len(tabs) == 2 && tabs[0].State == TabExpired && tabs[1].State == TabConnected
	})
	select {
	case <-server.done:
		t.Fatal("server stopped while a second tab remained connected")
	default:
	}
	clock.advance(now.Add(11 * time.Second))
	waitFor(t, func() bool {
		tabs := server.Tabs()
		return len(tabs) == 2 && tabs[0].State == TabExpired && tabs[1].State == TabExpired
	})
	clock.advance(now.Add(16 * time.Second))
	result := waitForResult(t, launch)
	if result.Reason != ShutdownBrowserDisconnected {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestAutomaticShutdownGuardRetries(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	var calls atomic.Int32
	reasons := make(chan ShutdownReason, 2)
	guard := ShutdownGuardFunc(func(_ context.Context, request ShutdownRequest) error {
		reasons <- request.Reason
		if calls.Add(1) == 1 {
			return DenyShutdown("busy", "Still working")
		}
		return nil
	})
	server := newTestServerWithClock(t, clock, LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: time.Second,
		HeartbeatTimeout:    time.Second,
		CheckInterval:       time.Second,
	}, guard)
	launch := startWithTicker(t, server, clock)
	clock.advance(now.Add(time.Second))
	waitFor(t, func() bool { return calls.Load() == 1 })
	select {
	case <-server.done:
		t.Fatal("server stopped after denied automatic shutdown")
	default:
	}
	clock.advance(now.Add(2 * time.Second))
	result := waitForResult(t, launch)
	if result.Reason != ShutdownFirstContactTimeout || calls.Load() != 2 {
		t.Fatalf("result = %#v, calls = %d", result, calls.Load())
	}
	close(reasons)
	for reason := range reasons {
		if reason != ShutdownFirstContactTimeout {
			t.Fatalf("guard reason = %q", reason)
		}
	}
}

func TestReconnectDuringDisconnectGraceCancelsPendingShutdown(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server := newTestServerWithClock(t, clock, LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		DisconnectGrace:     5 * time.Second,
		CheckInterval:       time.Second,
	}, nil)
	launch := startWithTicker(t, server, clock)
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	tabID := testTabID(9)
	if err := server.tabs.heartbeat(tabID, now); err != nil {
		t.Fatal(err)
	}
	if err := server.tabs.disconnect(tabID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	clock.advance(now.Add(time.Second))
	waitFor(t, func() bool { return len(server.Tabs()) == 1 && server.Tabs()[0].State == TabDisconnected })
	if err := server.tabs.heartbeat(tabID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	clock.advance(now.Add(7 * time.Second))
	select {
	case <-server.done:
		t.Fatal("server stopped despite reconnect during grace")
	default:
	}
}

func TestExplicitLifetimeNeverStopsForBrowserAbsence(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server := newTestServerWithClock(t, clock, ExplicitLifetime(), nil)
	launch, err := server.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.tabs.heartbeat(testTabID(3), now); err != nil {
		t.Fatal(err)
	}
	if err := server.tabs.disconnect(testTabID(3), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	clock.advance(now.Add(time.Hour))
	select {
	case <-server.done:
		t.Fatal("explicit lifetime stopped because browser was absent")
	default:
	}
	if err := launch.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := launch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != ShutdownProgrammatic {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func newTestServerWithClock(t *testing.T, clock clock, lifetime LifetimePolicy, guard ShutdownGuard) *Server {
	t.Helper()
	server, err := newServer(Options{
		Handler:  http.NotFoundHandler(),
		Lifetime: lifetime,
		Guard:    guard,
	}, clock, nil, strings.NewReader(strings.Repeat("t", 112)))
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func startWithTicker(t *testing.T, server *Server, clock *manualClock) *Launch {
	t.Helper()
	launch, err := server.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-clock.created:
	case <-time.After(time.Second):
		t.Fatal("lifetime ticker was not created")
	}
	return launch
}

func waitForResult(t *testing.T, launch *Launch) Result {
	t.Helper()
	type outcome struct {
		result Result
		err    error
	}
	resultChannel := make(chan outcome, 1)
	go func() {
		result, err := launch.Wait()
		resultChannel <- outcome{result: result, err: err}
	}()
	select {
	case received := <-resultChannel:
		if received.err != nil {
			t.Fatal(received.err)
		}
		return received.result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
		return Result{}
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not satisfied")
		}
		time.Sleep(time.Millisecond)
	}
}
