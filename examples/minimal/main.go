// Command minimal is an executable specification of Singleserve's public API.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rztaylor/singleserve"
)

//go:embed static/index.html
var indexHTML []byte

//go:embed static/styles.css
var stylesCSS []byte

//go:embed static/app.js
var appJS []byte

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout, os.Stderr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "minimal:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout, stderr io.Writer, opener singleserve.BrowserOpener) error {
	lifetime := singleserve.BrowserBoundLifetime()
	options := singleserve.Options{
		Handler:  applicationHandler(lifetime),
		Lifetime: lifetime,
	}
	if opener != nil {
		options.Opener = opener
	}

	server, err := singleserve.New(options)
	if err != nil {
		return err
	}
	launch, err := server.Start(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "Singleserve minimal is running on loopback", launch.Address())
	if err := launch.OpenBrowser(ctx); err != nil {
		fmt.Fprintln(stderr, "Could not open a browser:", err)
		fmt.Fprintln(stderr, "Open this URL manually:", launch.URL())
	}

	result, err := launch.Wait()
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Singleserve minimal stopped:", result.Reason)
	return nil
}

func applicationHandler(lifetime singleserve.LifetimePolicy) http.Handler {
	page := strings.NewReplacer(
		"__HEARTBEAT_TIMEOUT_MS__", milliseconds(lifetime.HeartbeatTimeout),
		"__DISCONNECT_GRACE_MS__", milliseconds(lifetime.DisconnectGrace),
		"__CHECK_INTERVAL_MS__", milliseconds(lifetime.CheckInterval),
	).Replace(string(indexHTML))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, page)
	})
	mux.HandleFunc("GET /styles.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(stylesCSS)
	})
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(appJS)
	})
	mux.HandleFunc("GET /api/greeting", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Hello from Singleserve"})
	})
	return mux
}

func milliseconds(duration time.Duration) string {
	return strconv.FormatInt(duration.Milliseconds(), 10)
}
