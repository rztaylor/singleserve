package singleserve

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProgrammaticShutdownDrainsInFlightRequestAndBypassesGuard(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server, err := New(Options{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusNoContent)
		}),
		Guard: ShutdownGuardFunc(func(context.Context, ShutdownRequest) error {
			return DenyShutdown("always", "Never allow browser shutdown")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	requestDone := make(chan error, 1)
	go func() {
		response, requestErr := launch.Client().Get(launch.BaseURL())
		if requestErr == nil {
			requestErr = response.Body.Close()
		}
		requestDone <- requestErr
	}()
	<-started
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- launch.Shutdown(context.Background()) }()
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown returned before in-flight request completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	if err := <-shutdownDone; err != nil {
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

func TestNewBootstrapURLRequiresRunningLaunch(t *testing.T) {
	var nilLaunch *Launch
	if _, err := nilLaunch.NewBootstrapURL(); err == nil || !strings.Contains(err.Error(), "nil launch") {
		t.Fatalf("nil launch renewal error = %v", err)
	}

	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := launch.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.NewBootstrapURL(); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("stopped launch renewal error = %v", err)
	}
}

func TestDrainTimeoutIsReturned(t *testing.T) {
	started := make(chan struct{})
	blocked := make(chan struct{})
	server, err := New(Options{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-blocked
	})})
	if err != nil {
		t.Fatal(err)
	}
	server.drainTimeout = 10 * time.Millisecond
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	requestDone := make(chan struct{})
	go func() {
		response, requestErr := launch.Client().Get(launch.BaseURL())
		if requestErr == nil {
			_ = response.Body.Close()
		}
		close(requestDone)
	}()
	<-started
	err = launch.Shutdown(context.Background())
	if err == nil || !strings.Contains(err.Error(), "graceful shutdown") {
		t.Fatalf("Shutdown error = %v", err)
	}
	close(blocked)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("blocked request did not finish after forced close")
	}
	result, waitErr := launch.Wait()
	if waitErr == nil || result.Reason != ShutdownProgrammatic {
		t.Fatalf("Wait = %#v, %v", result, waitErr)
	}
}

func TestConcurrentRequestsAndShutdownArbitration(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	client := launch.Client()
	var requests sync.WaitGroup
	requestErrors := make(chan error, 32)
	for range 32 {
		requests.Add(1)
		go func() {
			defer requests.Done()
			request, requestErr := http.NewRequest(http.MethodGet, launch.BaseURL()+"_singleserve/health", nil)
			if requestErr != nil {
				requestErrors <- requestErr
				return
			}
			response, requestErr := client.Do(request)
			if requestErr != nil {
				requestErrors <- requestErr
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				requestErrors <- fmt.Errorf("health status = %d", response.StatusCode)
			}
		}()
	}
	requests.Wait()
	close(requestErrors)
	for requestErr := range requestErrors {
		t.Error(requestErr)
	}

	reasons := []ShutdownReason{ShutdownProgrammatic, ShutdownContextCanceled, ShutdownBrowserDisconnected}
	winners := make(chan ShutdownReason, len(reasons))
	var shutdowns sync.WaitGroup
	for _, reason := range reasons {
		shutdowns.Add(1)
		go func() {
			defer shutdowns.Done()
			if server.beginStop(reason) {
				winners <- reason
			}
		}()
	}
	shutdowns.Wait()
	close(winners)
	var winner ShutdownReason
	for reason := range winners {
		if winner != "" {
			t.Fatalf("multiple shutdown winners: %q and %q", winner, reason)
		}
		winner = reason
	}
	result, err := launch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if winner == "" || result.Reason != winner {
		t.Fatalf("winner = %q, result = %#v", winner, result)
	}
}

func TestStartSingleUseAndParentCancellation(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	launch, err := server.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second Start error = %v", err)
	}
	cancel()
	result, err := launch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != ShutdownContextCanceled {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestCanceledContextDoesNotBind(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var listened atomic.Bool
	server, err := newServer(Options{Handler: http.NotFoundHandler()}, realClock{}, func(string, string) (net.Listener, error) {
		listened.Store(true)
		return nil, errors.New("unexpected")
	}, strings.NewReader(strings.Repeat("x", 112)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start error = %v", err)
	}
	if listened.Load() {
		t.Fatal("listener called for canceled context")
	}
}

func TestConstructionAndListenFailures(t *testing.T) {
	if _, err := New(Options{}); err == nil || !strings.Contains(err.Error(), "handler is required") {
		t.Fatalf("missing handler error = %v", err)
	}
	for _, test := range []struct {
		bytes int
		want  string
	}{
		{bytes: 0, want: "generate bootstrap credential"},
		{bytes: 32, want: "generate browser session credential"},
		{bytes: 64, want: "generate programmatic credential"},
		{bytes: 96, want: "generate launch origin"},
	} {
		if _, err := newServer(Options{Handler: http.NotFoundHandler()}, realClock{}, nil, strings.NewReader(strings.Repeat("x", test.bytes))); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("entropy bytes %d error = %v, want %q", test.bytes, err, test.want)
		}
	}
	want := errors.New("address unavailable")
	server, err := newServer(Options{Handler: http.NotFoundHandler()}, realClock{}, func(string, string) (net.Listener, error) {
		return nil, want
	}, strings.NewReader(strings.Repeat("n", 112)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Start(context.Background()); !errors.Is(err, want) {
		t.Fatalf("listen error = %v", err)
	}
	if _, err := server.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second Start after listen failure = %v", err)
	}
}

type failingListener struct {
	address net.Addr
	err     error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (l failingListener) Close() error              { return nil }
func (l failingListener) Addr() net.Addr            { return l.address }

func TestUnexpectedServeFailureIsReported(t *testing.T) {
	want := errors.New("accept failed")
	server, err := newServer(Options{Handler: http.NotFoundHandler()}, realClock{}, func(string, string) (net.Listener, error) {
		return failingListener{address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 45678}, err: want}, nil
	}, strings.NewReader(strings.Repeat("e", 112)))
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := launch.Wait()
	if !errors.Is(err, want) {
		t.Fatalf("Wait error = %v", err)
	}
	if result.Reason != ShutdownServerError {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestHTTPServerSecurityBounds(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	server.mu.Lock()
	httpServer := server.httpServer
	server.mu.Unlock()
	if httpServer.ReadHeaderTimeout != 5*time.Second || httpServer.IdleTimeout != defaultIdleTimeout || httpServer.MaxHeaderBytes != defaultMaxHeaderBytes {
		t.Fatalf("HTTP server bounds = header %s, idle %s, bytes %d", httpServer.ReadHeaderTimeout, httpServer.IdleTimeout, httpServer.MaxHeaderBytes)
	}
}

func TestOversizedRequestHeadersNeverReachApplication(t *testing.T) {
	var applicationRequests atomic.Int32
	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		applicationRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	connection, err := net.DialTimeout("tcp", launch.Address(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	host := strings.TrimSuffix(strings.TrimPrefix(launch.BaseURL(), "http://"), "/")
	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nX-Oversized: %s\r\n\r\n", host, strings.Repeat("a", defaultMaxHeaderBytes+(16<<10)))
	if _, err := io.WriteString(connection, request); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge || applicationRequests.Load() != 0 {
		t.Fatalf("oversized header response = %d, application requests = %d", response.StatusCode, applicationRequests.Load())
	}
}
