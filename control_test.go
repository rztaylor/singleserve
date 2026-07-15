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
	"sync/atomic"
	"testing"
	"time"
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
	tabID := testTabID(7)
	headers := http.Header{TabHeader: []string{tabID}}
	client := launch.Client()

	heartbeat := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/tabs/heartbeat", headers)
	if heartbeat.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d", heartbeat.StatusCode)
	}
	closeBody(t, heartbeat)
	if tabs := server.Tabs(); len(tabs) != 1 || tabs[0].ID != tabID || tabs[0].State != TabConnected {
		t.Fatalf("tabs after heartbeat = %#v", tabs)
	}

	shutdown := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", headers)
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

	disconnect := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/tabs/disconnect", headers)
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
	response := mustRequest(t, launch.Client(), http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", nil)
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
	response := mustRequest(t, launch.Client(), http.MethodPost, launch.BaseURL()+"_singleserve/shutdown", nil)
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("guard failure status = %d", response.StatusCode)
	}
	body := readBody(t, response)
	if !strings.Contains(body, `"code":"shutdown_guard_failed"`) || strings.Contains(body, "private detail") {
		t.Fatalf("guard failure body = %s", body)
	}
}

func TestControlEndpointsRejectEveryRequestBodyForm(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	tests := []struct {
		name      string
		body      io.Reader
		configure func(*http.Request)
	}{
		{name: "known length", body: strings.NewReader("payload")},
		{name: "chunked", body: strings.NewReader("payload"), configure: func(request *http.Request) {
			request.ContentLength = -1
			request.TransferEncoding = []string{"chunked"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, requestErr := http.NewRequest(http.MethodGet, launch.BaseURL()+"_singleserve/health", test.body)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			if test.configure != nil {
				test.configure(request)
			}
			response, requestErr := launch.Client().Do(request)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			if response.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d", response.StatusCode)
			}
			closeBody(t, response)
		})
	}
}

func TestControlBodyRejectionDoesNotDrainSlowClient(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
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
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	host := strings.TrimSuffix(strings.TrimPrefix(launch.BaseURL(), "http://"), "/")
	request := fmt.Sprintf("POST /_singleserve/health HTTP/1.1\r\nHost: %s\r\n%s: %s\r\nContent-Length: 10\r\n\r\n", host, TokenHeader, server.programToken)
	if _, err := io.WriteString(connection, request); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge || !response.Close {
		t.Fatalf("slow-body response = %d, close = %v", response.StatusCode, response.Close)
	}
}
