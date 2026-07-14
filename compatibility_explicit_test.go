package singleserve_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/rztaylor/singleserve"
)

// TestExplicitConsumerHarness exercises an owner-controlled application using
// only Singleserve's exported API. It represents custom routes and a shutdown
// guard that vetoes a browser request while allowing owner shutdown.
func TestExplicitConsumerHarness(t *testing.T) {
	routes := http.NewServeMux()
	guardRequests := make(chan singleserve.ShutdownRequest, 1)
	routes.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "explicit-lifetime application")
	})
	routes.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"status":"ready"}`)
	})
	server, err := singleserve.New(singleserve.Options{
		Handler:  routes,
		Lifetime: singleserve.ExplicitLifetime(),
		Guard: singleserve.ShutdownGuardFunc(func(_ context.Context, request singleserve.ShutdownRequest) error {
			guardRequests <- request
			return singleserve.DenyShutdown("work_in_progress", "Finish the current task first")
		}),
		Opener: singleserve.BrowserOpenerFunc(func(context.Context, string) error { return nil }),
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

	client := browserClient(t)
	baseURL, token := bootstrapConsumer(t, client, launch.URL())
	response, err := client.Get(baseURL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("application route status = %d", response.StatusCode)
	}

	response = controlRequest(t, client, http.MethodPost, baseURL+"/_singleserve/shutdown", token, tabID(2))
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("browser shutdown status = %d", response.StatusCode)
	}
	if request := <-guardRequests; request.Reason != singleserve.ShutdownBrowserRequest {
		t.Fatalf("guard reason = %q", request.Reason)
	}

	if err := launch.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := launch.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != singleserve.ShutdownProgrammatic {
		t.Fatalf("shutdown reason = %q", result.Reason)
	}
}
