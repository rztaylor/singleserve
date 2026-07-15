//go:build browsersecurity

package singleserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const browserFixtureHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Singleserve browser security fixture</title></head>
<body><p id="status">Loading</p>
<script type="module">
import { connect } from "/_singleserve/client.js";
window.__consumerURL = location.href;
window.__consumerReferrer = document.referrer;
try {
  const session = await connect({
    heartbeatIntervalMS: 100,
    heartbeatTimeoutMS: 500,
    failureThreshold: 3,
    onServerUnavailable() { window.__unavailable = true; }
  });
  window.__sessionKeys = Object.keys(session).sort();
  window.__healthy = await session.health();
  window.__requestShutdown = async () => {
    try {
      return await session.requestShutdown();
    } catch (error) {
      return {ok: false, status: error.status, code: error.code, message: error.message};
    }
  };
  window.__disconnectAndStop = async () => {
    await session.disconnect();
    session.stop();
  };
  window.__ready = true;
  document.querySelector("#status").textContent = "Ready";
} catch (error) {
  window.__failure = String(error?.message || error);
  document.querySelector("#status").textContent = "Failed";
}
</script></body></html>`

func TestRealBrowserSecurity(t *testing.T) {
	if os.Getenv("SINGLESERVE_BROWSER_SECURITY") != "1" {
		t.Fatal("real-browser security test requires SINGLESERVE_BROWSER_SECURITY=1")
	}
	driver := startWebDriver(t)
	defer driver.close(t)

	var firstReferer atomic.Value
	lifetime := LifetimePolicy{
		Mode:                LifetimeBrowserBound,
		FirstContactTimeout: 10 * time.Second,
		HeartbeatTimeout:    500 * time.Millisecond,
		DisconnectGrace:     2 * time.Second,
		CheckInterval:       50 * time.Millisecond,
	}
	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		firstReferer.CompareAndSwap(nil, r.Referer())
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, browserFixtureHTML)
	}), Lifetime: lifetime, Guard: ShutdownGuardFunc(func(_ context.Context, request ShutdownRequest) error {
		if request.Reason == ShutdownBrowserRequest {
			return DenyShutdown("browser_denied", "Browser shutdown is denied by the test guard")
		}
		return nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	session := driver.newSession(t)
	defer session.delete(t)
	session.navigate(t, launch.URL())
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)

	state := session.executeObject(t, `return {
  href: location.href,
  hash: location.hash,
  pathname: location.pathname,
  referrer: window.__consumerReferrer,
  consumerURL: window.__consumerURL,
  cookie: document.cookie,
  localKeys: Object.keys(localStorage),
  sessionKeys: Object.keys(sessionStorage),
  apiKeys: window.__sessionKeys,
  healthy: window.__healthy
}`)
	if state["href"] != launch.BaseURL() || state["consumerURL"] != launch.BaseURL() || state["hash"] != "" || state["pathname"] != "/" {
		t.Fatalf("consumer page was not clean: %#v", state)
	}
	if state["referrer"] != "" || firstReferer.Load() != "" {
		t.Fatalf("bootstrap leaked a referrer: browser=%q server=%q", state["referrer"], firstReferer.Load())
	}
	if state["cookie"] != "" || !emptyJSONArray(state["localKeys"]) || !emptyJSONArray(state["sessionKeys"]) {
		t.Fatalf("browser-readable persistence was populated: %#v", state)
	}
	if state["healthy"] != true || strings.Join(arrayStrings(state["apiKeys"]), ",") != "disconnect,fetch,health,requestShutdown,stop,tabID" {
		t.Fatalf("browser session API was unhealthy or exposed a token: %#v", state)
	}
	shutdown := session.executeAsyncObject(t, `const done = arguments[arguments.length - 1]; window.__requestShutdown().then(done);`)
	if shutdown["ok"] != false || shutdown["status"] != float64(http.StatusConflict) || shutdown["code"] != "browser_denied" {
		t.Fatalf("guarded browser shutdown result = %#v", shutdown)
	}
	cookies := session.cookies(t)
	if len(cookies) != 1 || !strings.HasPrefix(fmt.Sprint(cookies[0]["name"]), "__Host-") || cookies[0]["secure"] != true || cookies[0]["httpOnly"] != true || cookies[0]["sameSite"] != "Strict" || cookies[0]["path"] != "/" {
		t.Fatalf("browser did not retain the required Secure __Host- session cookie: %#v", cookies)
	}
	persistence := session.executeAsyncObject(t, `const done = arguments[arguments.length - 1];
Promise.all([
  globalThis.indexedDB?.databases ? indexedDB.databases().then(values => values.map(value => value.name)) : [],
  globalThis.caches?.keys ? caches.keys() : [],
  navigator.serviceWorker?.getRegistrations ? navigator.serviceWorker.getRegistrations().then(values => values.map(value => value.scope)) : []
]).then(([databases, caches, workers]) => done({databases, caches, workers}), error => done({error: String(error)}));`)
	if persistence["error"] != nil || !emptyJSONArray(persistence["databases"]) || !emptyJSONArray(persistence["caches"]) || !emptyJSONArray(persistence["workers"]) {
		t.Fatalf("persistent browser state was created: %#v", persistence)
	}

	session.refresh(t)
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	if result := session.execute(t, `return window.__healthy === true && location.href === arguments[0]`, launch.BaseURL()); result != true {
		t.Fatalf("cookie-authenticated reload result = %#v", result)
	}

	originalHandle := session.windowHandle(t)
	newHandle := session.newTab(t)
	session.switchTo(t, newHandle)
	session.navigate(t, launch.BaseURL())
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	waitForCondition(t, 5*time.Second, func() bool { return connectedTabCount(server.Tabs()) >= 2 })
	session.closeWindow(t)
	session.switchTo(t, originalHandle)
	waitForCondition(t, 5*time.Second, func() bool { return connectedTabCount(server.Tabs()) == 1 })

	seenCookie := make(chan string, 2)
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookie <- r.Header.Get("Cookie")
		cookie := &http.Cookie{Name: server.cookie, Value: "hostile", Path: "/", Secure: true}
		if strings.HasPrefix(strings.ToLower(r.Host), "evil.localhost:") {
			cookie.Domain = "localhost"
		}
		http.SetCookie(w, cookie)
		_, _ = io.WriteString(w, "hostile sibling")
	}))
	defer hostile.Close()
	session.navigate(t, hostile.URL)
	if cookie := <-seenCookie; cookie != "" {
		t.Fatalf("sibling loopback origin received launch cookie %q", cookie)
	}
	session.navigate(t, launch.BaseURL())
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	if result := session.execute(t, `return window.__healthy === true`); result != true {
		t.Fatalf("sibling origin overwrote launch session: %#v", result)
	}
	hostileURL := strings.Replace(hostile.URL, "127.0.0.1", "evil.localhost", 1)
	session.navigate(t, hostileURL)
	if cookie := <-seenCookie; cookie != "" {
		t.Fatalf("related .localhost origin received launch cookie %q", cookie)
	}
	session.navigate(t, launch.BaseURL())
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	if result := session.execute(t, `return window.__healthy === true && document.cookie === ""`); result != true {
		t.Fatalf("related-domain cookie injection changed launch session: %#v", result)
	}

	session.deleteAllCookies(t)
	session.navigate(t, launch.URL())
	session.waitFor(t, `return document.querySelector("#status")?.textContent.includes("invalid or expired") === true`, 15*time.Second)
	replay := session.executeObject(t, `return {href: location.href, hash: location.hash, ready: window.__ready === true, text: document.querySelector("#status")?.textContent}`)
	if replay["hash"] != "" || replay["ready"] == true || !strings.Contains(fmt.Sprint(replay["text"]), "invalid or expired") {
		t.Fatalf("consumed bootstrap replay was not rejected cleanly: %#v", replay)
	}
	renewedURL, err := launch.NewBootstrapURL()
	if err != nil {
		t.Fatal(err)
	}
	session.navigate(t, renewedURL)
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	if result := session.execute(t, `return window.__healthy === true && location.href === arguments[0] && location.hash === ""`, launch.BaseURL()); result != true {
		t.Fatalf("renewed bootstrap result = %#v", result)
	}
	disconnected := session.executeAsyncObject(t, `const done = arguments[arguments.length - 1]; window.__disconnectAndStop().then(() => done({ok: true}), error => done({error: String(error)}));`)
	if disconnected["ok"] != true {
		t.Fatalf("renewed browser disconnect result = %#v", disconnected)
	}
	result := waitForLaunchResult(t, launch, 5*time.Second)
	if result.Reason != ShutdownBrowserDisconnected {
		t.Fatalf("final browser tab shutdown reason = %q", result.Reason)
	}

	unavailableServer, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, browserFixtureHTML)
	})})
	if err != nil {
		t.Fatal(err)
	}
	unavailableLaunch, err := unavailableServer.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	session.navigate(t, unavailableLaunch.URL())
	session.waitFor(t, `return window.__ready === true`, 15*time.Second)
	if err := unavailableLaunch.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	session.waitFor(t, `return window.__unavailable === true`, 10*time.Second)
}

type webDriver struct {
	baseURL string
	client  *http.Client
	command *exec.Cmd
	output  *bytes.Buffer
}

func startWebDriver(t *testing.T) *webDriver {
	t.Helper()
	playwrightDriver := os.Getenv("SINGLESERVE_PLAYWRIGHT_DRIVER")
	path := os.Getenv("SINGLESERVE_WEBDRIVER")
	if playwrightDriver == "" && path == "" {
		var err error
		for _, candidate := range []string{"chromedriver", "geckodriver", "safaridriver"} {
			path, err = exec.LookPath(candidate)
			if err == nil {
				break
			}
		}
		if path == "" {
			t.Fatal("no browser automation backend found; set SINGLESERVE_PLAYWRIGHT_DRIVER or SINGLESERVE_WEBDRIVER")
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	var command *exec.Cmd
	if playwrightDriver != "" {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Fatal("Playwright browser security checks require Node.js")
		}
		command = exec.CommandContext(t.Context(), node, playwrightDriver, fmt.Sprintf("--port=%d", port))
	} else {
		name := strings.ToLower(filepath.Base(path))
		var args []string
		switch {
		case strings.Contains(name, "safari"):
			args = []string{"--port", fmt.Sprint(port)}
		case strings.Contains(name, "gecko"):
			args = []string{"--port", fmt.Sprint(port)}
		default:
			args = []string{fmt.Sprintf("--port=%d", port), "--allowed-ips=127.0.0.1"}
		}
		command = exec.CommandContext(t.Context(), path, args...)
	}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatalf("start WebDriver: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	driver := &webDriver{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client:  &http.Client{Transport: transport, Timeout: 10 * time.Second},
		command: command,
		output:  &output,
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := driver.client.Get(driver.baseURL + "/status")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return driver
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	driver.close(t)
	t.Fatalf("WebDriver did not become ready: %s", output.String())
	return nil
}

func (d *webDriver) close(t *testing.T) {
	t.Helper()
	if d == nil || d.command == nil || d.command.Process == nil {
		return
	}
	_ = d.command.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- d.command.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = d.command.Process.Kill()
		<-done
	}
}

type webDriverSession struct {
	driver *webDriver
	id     string
}

func (d *webDriver) newSession(t *testing.T) *webDriverSession {
	t.Helper()
	browserName := os.Getenv("SINGLESERVE_BROWSER_NAME")
	if browserName == "" {
		if os.Getenv("SINGLESERVE_PLAYWRIGHT_DRIVER") != "" {
			browserName = "chromium"
		}
		name := strings.ToLower(filepath.Base(d.command.Path))
		if browserName == "" {
			switch {
			case strings.Contains(name, "safari"):
				browserName = "safari"
			case strings.Contains(name, "gecko"):
				browserName = "firefox"
			default:
				browserName = "chrome"
			}
		}
	}
	alwaysMatch := map[string]any{"browserName": browserName}
	if browserName == "chrome" || browserName == "chromium" {
		chromeOptions := map[string]any{"args": []string{"--headless=new", "--disable-gpu", "--no-sandbox"}}
		if binary := os.Getenv("SINGLESERVE_BROWSER_BINARY"); binary != "" {
			chromeOptions["binary"] = binary
		}
		alwaysMatch["goog:chromeOptions"] = chromeOptions
	}
	var value map[string]any
	d.request(t, http.MethodPost, "/session", map[string]any{"capabilities": map[string]any{"alwaysMatch": alwaysMatch}}, &value)
	id, _ := value["sessionId"].(string)
	if id == "" {
		t.Fatalf("WebDriver returned no session ID: %#v", value)
	}
	return &webDriverSession{driver: d, id: id}
}

func (d *webDriver) request(t *testing.T, method, path string, payload any, result any) {
	t.Helper()
	if err := d.requestError(t, method, path, payload, result); err != nil {
		t.Fatal(err)
	}
}

func (d *webDriver) requestError(t *testing.T, method, path string, payload any, result any) error {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(t.Context(), method, d.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode WebDriver %s %s response: %w", method, path, err)
	}
	var failure struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(envelope.Value, &failure)
	if response.StatusCode >= 300 || failure.Error != "" {
		return fmt.Errorf("WebDriver %s %s failed: %s: %s\n%s", method, path, failure.Error, failure.Message, d.output.String())
	}
	if result != nil && string(envelope.Value) != "null" {
		if err := json.Unmarshal(envelope.Value, result); err != nil {
			return fmt.Errorf("decode WebDriver value: %w", err)
		}
	}
	return nil
}

func (s *webDriverSession) path(suffix string) string { return "/session/" + s.id + suffix }

func (s *webDriverSession) delete(t *testing.T) {
	t.Helper()
	s.driver.request(t, http.MethodDelete, s.path(""), nil, nil)
}

func (s *webDriverSession) navigate(t *testing.T, rawURL string) {
	t.Helper()
	s.driver.request(t, http.MethodPost, s.path("/url"), map[string]any{"url": rawURL}, nil)
}

func (s *webDriverSession) refresh(t *testing.T) {
	t.Helper()
	s.driver.request(t, http.MethodPost, s.path("/refresh"), map[string]any{}, nil)
}

func (s *webDriverSession) execute(t *testing.T, script string, args ...any) any {
	t.Helper()
	result, err := s.executeResult(t, script, args...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (s *webDriverSession) executeResult(t *testing.T, script string, args ...any) (any, error) {
	t.Helper()
	var result any
	err := s.driver.requestError(t, http.MethodPost, s.path("/execute/sync"), map[string]any{"script": script, "args": args}, &result)
	return result, err
}

func (s *webDriverSession) executeObject(t *testing.T, script string, args ...any) map[string]any {
	t.Helper()
	result := s.execute(t, script, args...)
	object, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("script result = %#v", result)
	}
	return object
}

func (s *webDriverSession) executeAsyncObject(t *testing.T, script string) map[string]any {
	t.Helper()
	var result map[string]any
	s.driver.request(t, http.MethodPost, s.path("/execute/async"), map[string]any{"script": script, "args": []any{}}, &result)
	return result
}

func (s *webDriverSession) waitFor(t *testing.T, script string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		result, err := s.executeResult(t, script)
		if err == nil && result == true {
			return
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("browser condition timed out: %s; last browser error: %v", script, lastErr)
	}
	t.Fatalf("browser condition timed out: %s", script)
}

func (s *webDriverSession) windowHandle(t *testing.T) string {
	t.Helper()
	var handle string
	s.driver.request(t, http.MethodGet, s.path("/window"), nil, &handle)
	return handle
}

func (s *webDriverSession) newTab(t *testing.T) string {
	t.Helper()
	var result struct {
		Handle string `json:"handle"`
	}
	s.driver.request(t, http.MethodPost, s.path("/window/new"), map[string]any{"type": "tab"}, &result)
	return result.Handle
}

func (s *webDriverSession) switchTo(t *testing.T, handle string) {
	t.Helper()
	s.driver.request(t, http.MethodPost, s.path("/window"), map[string]any{"handle": handle}, nil)
}

func (s *webDriverSession) closeWindow(t *testing.T) {
	t.Helper()
	s.driver.request(t, http.MethodDelete, s.path("/window"), nil, nil)
}

func (s *webDriverSession) deleteAllCookies(t *testing.T) {
	t.Helper()
	s.driver.request(t, http.MethodDelete, s.path("/cookie"), nil, nil)
}

func (s *webDriverSession) cookies(t *testing.T) []map[string]any {
	t.Helper()
	var cookies []map[string]any
	s.driver.request(t, http.MethodGet, s.path("/cookie"), nil, &cookies)
	return cookies
}

func emptyJSONArray(value any) bool {
	values, ok := value.([]any)
	return ok && len(values) == 0
}

func arrayStrings(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil
		}
		result = append(result, text)
	}
	return result
}

func connectedTabCount(tabs []TabSnapshot) int {
	count := 0
	for _, tab := range tabs {
		if tab.State == TabConnected {
			count++
		}
	}
	return count
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}

func waitForLaunchResult(t *testing.T, launch *Launch, timeout time.Duration) Result {
	t.Helper()
	results := make(chan Result, 1)
	errors := make(chan error, 1)
	go func() {
		result, err := launch.Wait()
		if err != nil {
			errors <- err
			return
		}
		results <- result
	}()
	select {
	case result := <-results:
		return result
	case err := <-errors:
		t.Fatal(err)
	case <-time.After(timeout):
		t.Fatal("timed out waiting for browser-bound shutdown")
	}
	return Result{}
}
