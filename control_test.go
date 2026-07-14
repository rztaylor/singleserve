package singleserve

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestControlTabsAndGuardedShutdown(t *testing.T) {
	var guardCalls atomic.Int32
	guardRequests := make(chan ShutdownRequest, 1)
	server, err := New(Options{
		Handler: http.NotFoundHandler(),
		Guard: ShutdownGuardFunc(func(_ context.Context, request ShutdownRequest) error {
			guardCalls.Add(1)
			guardRequests <- request
			return DenyShutdown("unsaved_changes", "Save or discard your changes first")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	token := tokenFromLaunchURL(t, launch.URL())
	tabID := testTabID(7)
	headers := http.Header{TokenHeader: []string{token}, TabHeader: []string{tabID}}

	heartbeat := mustRequest(t, http.DefaultClient, http.MethodPost, launch.BaseURL()+"_singleserve/tabs/heartbeat", headers)
	if heartbeat.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d", heartbeat.StatusCode)
	}
	closeBody(t, heartbeat)
	if tabs := server.Tabs(); len(tabs) != 1 || tabs[0].ID != tabID || tabs[0].State != TabConnected {
		t.Fatalf("tabs after heartbeat = %#v", tabs)
	}

	shutdown := mustRequest(t, http.DefaultClient, http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", headers)
	if shutdown.StatusCode != http.StatusConflict {
		t.Fatalf("guarded shutdown status = %d, want 409", shutdown.StatusCode)
	}
	body := readBody(t, shutdown)
	if !strings.Contains(body, `"code":"unsaved_changes"`) {
		t.Fatalf("guard denial body = %s", body)
	}
	if guardCalls.Load() != 1 {
		t.Fatalf("guard calls = %d", guardCalls.Load())
	}
	request := <-guardRequests
	if request.Reason != ShutdownBrowserRequest || len(request.Tabs) != 1 || request.Tabs[0].State != TabConnected {
		t.Fatalf("guard request = %#v", request)
	}

	disconnect := mustRequest(t, http.DefaultClient, http.MethodPost, launch.BaseURL()+"_singleserve/tabs/disconnect", headers)
	closeBody(t, disconnect)
	if tabs := server.Tabs(); len(tabs) != 1 || tabs[0].State != TabDisconnected {
		t.Fatalf("tabs after disconnect = %#v", tabs)
	}
}

func TestBrowserShutdownCompletesWithReason(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	headers := http.Header{TokenHeader: []string{tokenFromLaunchURL(t, launch.URL())}}
	response := mustRequest(t, http.DefaultClient, http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", headers)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown status = %d", response.StatusCode)
	}
	closeBody(t, response)
	result, err := launch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != ShutdownBrowserRequest || result.StartedAt.IsZero() || result.StoppedAt.Before(result.StartedAt) {
		t.Fatalf("result = %#v", result)
	}
}

func TestUnexpectedGuardFailureIsGeneric(t *testing.T) {
	server, err := New(Options{
		Handler: http.NotFoundHandler(),
		Guard:   ShutdownGuardFunc(func(context.Context, ShutdownRequest) error { return errors.New("private detail") }),
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	headers := http.Header{TokenHeader: []string{tokenFromLaunchURL(t, launch.URL())}}
	response := mustRequest(t, http.DefaultClient, http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", headers)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("guard failure status = %d", response.StatusCode)
	}
	body := readBody(t, response)
	if !strings.Contains(body, `"code":"shutdown_guard_failed"`) || strings.Contains(body, "private detail") {
		t.Fatalf("guard failure body = %s", body)
	}
}
