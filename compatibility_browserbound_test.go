package singleserve_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rztaylor/singleserve"
)

// TestBrowserBoundConsumerHarness exercises a browser-owned application using
// only Singleserve's exported API. It represents a custom route plus a
// browser client that disconnects when its final tab closes.
func TestBrowserBoundConsumerHarness(t *testing.T) {
	opened := make(chan string, 1)
	server, err := singleserve.New(singleserve.Options{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprint(w, "browser-bound application")
		}),
		Lifetime: singleserve.LifetimePolicy{
			Mode:             singleserve.LifetimeBrowserBound,
			HeartbeatTimeout: time.Second,
			DisconnectGrace:  time.Millisecond,
			CheckInterval:    time.Millisecond,
		},
		Opener: singleserve.BrowserOpenerFunc(func(_ context.Context, rawURL string) error {
			opened <- rawURL
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	launch, err := server.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := launch.OpenBrowser(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-opened; got != launch.URL() {
		t.Fatalf("opened URL = %q, want %q", got, launch.URL())
	}

	client := browserClient(t)
	baseURL, token := bootstrapConsumer(t, client, launch.URL())
	tab := tabID(1)
	response := controlRequest(t, client, http.MethodPost, baseURL+"/_singleserve/tabs/heartbeat", token, tab)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d", response.StatusCode)
	}
	response = controlRequest(t, client, http.MethodPost, baseURL+"/_singleserve/tabs/disconnect", token, tab)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("disconnect status = %d", response.StatusCode)
	}

	type waitResult struct {
		result singleserve.Result
		err    error
	}
	finished := make(chan waitResult, 1)
	go func() {
		result, err := launch.Wait()
		finished <- waitResult{result: result, err: err}
	}()
	select {
	case completed := <-finished:
		if completed.err != nil {
			t.Fatal(completed.err)
		}
		if completed.result.Reason != singleserve.ShutdownBrowserDisconnected {
			t.Fatalf("shutdown reason = %q", completed.result.Reason)
		}
	case <-time.After(consumerHarnessTimeout):
		t.Fatal("browser-bound server did not stop after final tab disconnect")
	}
}
