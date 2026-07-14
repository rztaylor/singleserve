package singleserve

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type listenFunc func(network, address string) (net.Listener, error)

const defaultDrainTimeout = 5 * time.Second

// ErrAlreadyStarted reports an attempt to start a single-use Server twice.
var ErrAlreadyStarted = errors.New("singleserve: server already started")

// Options configures one single-use local server.
type Options struct {
	Handler  http.Handler
	Address  string
	Lifetime LifetimePolicy
	Guard    ShutdownGuard
	Opener   BrowserOpener
}

// Server owns one authenticated loopback listener and its lifecycle state.
// A Server is single-use.
type Server struct {
	handler      http.Handler
	address      string
	lifetime     LifetimePolicy
	guard        ShutdownGuard
	opener       BrowserOpener
	token        string
	cookie       string
	tabs         *tabRegistry
	clock        clock
	listen       listenFunc
	drainTimeout time.Duration

	mu           sync.Mutex
	started      bool
	startedAt    time.Time
	baseURL      string
	launchURL    string
	httpServer   *http.Server
	lifecycleCtx context.Context
	cancel       context.CancelFunc
	stopCh       chan struct{}
	done         chan struct{}
	stopping     bool
	outcome      waitOutcome
}

type waitOutcome struct {
	result Result
	err    error
}

// Result describes the completed server lifetime.
type Result struct {
	Reason    ShutdownReason
	StartedAt time.Time
	StoppedAt time.Time
}

// Launch is the active view of a started Server.
type Launch struct {
	server *Server

	mu            sync.Mutex
	browserOpened bool
}

// New validates options and prepares a single-use Server.
func New(options Options) (*Server, error) {
	return newServer(options, realClock{}, net.Listen, rand.Reader)
}

func newServer(options Options, runtimeClock clock, listen listenFunc, entropy io.Reader) (*Server, error) {
	if options.Handler == nil {
		return nil, fmt.Errorf("singleserve: handler is required")
	}
	address, err := normalizeAddress(options.Address)
	if err != nil {
		return nil, err
	}
	if err := options.Lifetime.validate(); err != nil {
		return nil, err
	}
	if runtimeClock == nil {
		runtimeClock = realClock{}
	}
	if listen == nil {
		listen = net.Listen
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	token, err := generateToken(entropy)
	if err != nil {
		return nil, err
	}
	opener := options.Opener
	if opener == nil {
		opener = systemBrowserOpener{}
	}
	return &Server{
		handler:      options.Handler,
		address:      address,
		lifetime:     options.Lifetime,
		guard:        options.Guard,
		opener:       opener,
		token:        token,
		cookie:       cookieName(token),
		tabs:         newTabRegistry(),
		clock:        runtimeClock,
		listen:       listen,
		drainTimeout: defaultDrainTimeout,
	}, nil
}

func generateToken(entropy io.Reader) (string, error) {
	var value [32]byte
	if _, err := io.ReadFull(entropy, value[:]); err != nil {
		return "", fmt.Errorf("singleserve: generate launch token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func cookieName(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "singleserve_" + hex.EncodeToString(digest[:12])
}

// Start binds the configured listener and starts serving before returning.
func (s *Server) Start(ctx context.Context) (*Launch, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil, ErrAlreadyStarted
	}
	s.started = true
	s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listener, err := s.listen("tcp", s.address)
	if err != nil {
		return nil, fmt.Errorf("singleserve: listen on %s: %w", s.address, err)
	}

	baseURL, launchURL, err := s.urls(listener.Addr())
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	startedAt := s.clock.Now()
	httpServer := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           s.authenticatedHandler(baseURL),
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.mu.Lock()
	s.startedAt = startedAt
	s.baseURL = baseURL
	s.launchURL = launchURL
	s.httpServer = httpServer
	s.lifecycleCtx = lifecycleCtx
	s.cancel = cancel
	s.stopCh = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	serveErr := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()
	go s.coordinate(serveErr)
	go s.watchParent(ctx)
	if s.lifetime.Mode == LifetimeBrowserBound {
		go s.watchLifetime(lifecycleCtx)
	}

	return &Launch{server: s}, nil
}

func (s *Server) urls(address net.Addr) (string, string, error) {
	if address == nil {
		return "", "", fmt.Errorf("singleserve: listener has no address")
	}
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return "", "", fmt.Errorf("singleserve: parse listener address: %w", err)
	}
	if host == "" || host == "::" {
		host = "127.0.0.1"
	}
	base := url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: "/"}
	launch := base
	query := launch.Query()
	query.Set("singleserve_token", s.token)
	launch.RawQuery = query.Encode()
	return base.String(), launch.String(), nil
}

func (s *Server) watchParent(parent context.Context) {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	select {
	case <-parent.Done():
		s.beginStop(ShutdownContextCanceled)
	case <-done:
	}
}

func (s *Server) beginStop(reason ShutdownReason) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || s.stopCh == nil || s.stopping {
		return false
	}
	s.stopping = true
	s.outcome.result.Reason = reason
	close(s.stopCh)
	s.cancel()
	return true
}

func (s *Server) coordinate(serveErr <-chan error) {
	s.mu.Lock()
	stopCh := s.stopCh
	httpServer := s.httpServer
	startedAt := s.startedAt
	s.mu.Unlock()

	var runErr error
	select {
	case err := <-serveErr:
		if err == nil {
			err = fmt.Errorf("singleserve: HTTP server stopped unexpectedly")
		}
		runErr = err
		s.recordServerFailure()
	case <-stopCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.drainTimeout)
		shutdownErr := httpServer.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			_ = httpServer.Close()
			runErr = fmt.Errorf("singleserve: graceful shutdown: %w", shutdownErr)
		}
		if err := <-serveErr; err != nil && runErr == nil {
			runErr = err
		}
	}

	s.mu.Lock()
	s.outcome.result.StartedAt = startedAt
	s.outcome.result.StoppedAt = s.clock.Now()
	s.outcome.err = runErr
	if s.cancel != nil {
		s.cancel()
	}
	close(s.done)
	s.mu.Unlock()
}

func (s *Server) recordServerFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopping {
		s.stopping = true
		s.outcome.result.Reason = ShutdownServerError
		if s.cancel != nil {
			s.cancel()
		}
	}
}

func (s *Server) wait() (Result, error) {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done == nil {
		return Result{}, fmt.Errorf("singleserve: server has not started")
	}
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outcome.result, s.outcome.err
}

// Tabs returns a time-ordered copy of the current tab registry.
func (s *Server) Tabs() []TabSnapshot {
	if s == nil || s.tabs == nil {
		return nil
	}
	return s.tabs.snapshots(s.clock.Now())
}

// Address returns the assigned listener address.
func (l *Launch) Address() string {
	if l == nil || l.server == nil {
		return ""
	}
	l.server.mu.Lock()
	defer l.server.mu.Unlock()
	if l.server.httpServer == nil {
		return ""
	}
	return l.server.httpServer.Addr
}

// BaseURL returns the listener URL without authentication material.
func (l *Launch) BaseURL() string {
	if l == nil || l.server == nil {
		return ""
	}
	l.server.mu.Lock()
	defer l.server.mu.Unlock()
	return l.server.baseURL
}

// URL returns the authenticated browser bootstrap URL. Treat it as a secret.
func (l *Launch) URL() string {
	if l == nil || l.server == nil {
		return ""
	}
	l.server.mu.Lock()
	defer l.server.mu.Unlock()
	return l.server.launchURL
}

// OpenBrowser opens URL with the configured platform opener.
func (l *Launch) OpenBrowser(ctx context.Context) error {
	if l == nil || l.server == nil {
		return fmt.Errorf("singleserve: nil launch")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.mu.Lock()
	if l.browserOpened {
		l.mu.Unlock()
		return ErrBrowserAlreadyOpened
	}
	l.browserOpened = true
	l.mu.Unlock()
	return l.server.opener.Open(ctx, l.URL())
}

// Shutdown forces owner-controlled graceful shutdown and waits for completion
// or for ctx to be canceled. It bypasses the application guard.
func (l *Launch) Shutdown(ctx context.Context) error {
	if l == nil || l.server == nil {
		return fmt.Errorf("singleserve: nil launch")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.server.beginStop(ShutdownProgrammatic)
	l.server.mu.Lock()
	done := l.server.done
	l.server.mu.Unlock()
	select {
	case <-done:
		_, err := l.server.wait()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait blocks until the HTTP server has fully stopped.
func (l *Launch) Wait() (Result, error) {
	if l == nil || l.server == nil {
		return Result{}, fmt.Errorf("singleserve: nil launch")
	}
	return l.server.wait()
}
