package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/singleserve"
)

const smokeTimeout = 5 * time.Second

func TestSmoke(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opened := make(chan string, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, &stdout, &stderr, singleserve.BrowserOpenerFunc(func(_ context.Context, rawURL string) error {
			opened <- rawURL
			return nil
		}))
	}()

	launchURL := receive(t, opened)
	parsed, err := url.Parse(launchURL)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Query().Get("singleserve_token")
	if token == "" {
		t.Fatal("browser opener received no launch token")
	}
	baseURL := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: "/"}).String()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: smokeTimeout}
	response := request(t, client, http.MethodGet, launchURL, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("frontend status = %d", response.StatusCode)
	}
	frontend := readBody(t, response)
	for _, fragment := range []string{
		`href="/styles.css"`,
		`src="/app.js"`,
		`id="terminal"`,
		`data-heartbeat-timeout-ms="15000"`,
		`data-disconnect-grace-ms="5000"`,
		`data-check-interval-ms="1000"`,
	} {
		if !strings.Contains(frontend, fragment) {
			t.Fatalf("frontend does not exercise lifecycle behavior %q", fragment)
		}
	}
	if strings.Contains(frontend, "__HEARTBEAT_TIMEOUT_MS__") {
		t.Fatal("frontend timing placeholders were not rendered")
	}
	response = request(t, client, http.MethodGet, baseURL+"styles.css", nil)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(readBody(t, response), ".heartbeat-copy") {
		t.Fatal("example stylesheet was not available to the frontend")
	}
	response = request(t, client, http.MethodGet, baseURL+"app.js", nil)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("example application module response = %d, %q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	applicationModule := readBody(t, response)
	for _, fragment := range []string{
		`from "/_singleserve/client.js"`,
		`onHeartbeat: showHeartbeat`,
		`session.requestShutdown()`,
		`window.close()`,
	} {
		if !strings.Contains(applicationModule, fragment) {
			t.Fatalf("application module does not exercise lifecycle behavior %q", fragment)
		}
	}
	response = request(t, client, http.MethodGet, baseURL+"_singleserve/client.js", nil)
	if response.StatusCode != http.StatusOK || !strings.Contains(readBody(t, response), "export async function connect") {
		t.Fatal("canonical browser client was not available to the frontend")
	}

	response = request(t, client, http.MethodGet, baseURL+"api/greeting", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("greeting status = %d", response.StatusCode)
	}
	var greeting map[string]string
	if err := json.NewDecoder(response.Body).Decode(&greeting); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if greeting["message"] != "Hello from Singleserve" {
		t.Fatalf("greeting = %#v", greeting)
	}

	headers := http.Header{
		singleserve.TokenHeader: []string{token},
		singleserve.TabHeader:   []string{base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16))},
		"Origin":                []string{strings.TrimSuffix(baseURL, "/")},
	}
	response = request(t, client, http.MethodPost, baseURL+"_singleserve/tabs/heartbeat", headers)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %d", response.StatusCode)
	}
	response.Body.Close()

	response = request(t, client, http.MethodPost, baseURL+"_singleserve/shutdown", headers)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown status = %d", response.StatusCode)
	}
	response.Body.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(smokeTimeout):
		t.Fatal("example did not stop after the browser shutdown request")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "Singleserve minimal is running at "+baseURL) || !strings.Contains(output, "Singleserve minimal stopped: browser_request") || strings.Contains(output, token) {
		t.Fatalf("stdout = %q", output)
	}
}

func TestBrowserLaunchFailureShowsManualURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var launchURL string
	err := run(ctx, &stdout, &stderr, singleserve.BrowserOpenerFunc(func(_ context.Context, rawURL string) error {
		launchURL = rawURL
		cancel()
		return errors.New("no browser available")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if launchURL == "" || !strings.Contains(stderr.String(), "Could not open a browser: no browser available") || !strings.Contains(stderr.String(), "Open this URL manually: "+launchURL) {
		t.Fatalf("launch URL = %q, stderr = %q", launchURL, stderr.String())
	}
	parsed, parseErr := url.Parse(launchURL)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if strings.Contains(stdout.String(), parsed.Query().Get("singleserve_token")) {
		t.Fatalf("stdout exposed launch token: %q", stdout.String())
	}
}

func request(t *testing.T, client *http.Client, method, rawURL string, headers http.Header) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if headers != nil {
		request.Header = headers.Clone()
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func receive(t *testing.T, values <-chan string) string {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(smokeTimeout):
		t.Fatal("example did not attempt to open a browser")
		return ""
	}
}
