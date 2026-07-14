package singleserve

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLaunchAuthenticationAndControlSurface(t *testing.T) {
	var appRequests atomic.Int32
	var receivedQuery string
	var receivedRequestURI string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appRequests.Add(1)
		receivedQuery = r.URL.RawQuery
		receivedRequestURI = r.RequestURI
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "application")
	})
	server, err := New(Options{Handler: handler})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	client := &http.Client{}
	response := mustRequest(t, client, http.MethodGet, launch.BaseURL(), nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	closeBody(t, response)
	if appRequests.Load() != 0 {
		t.Fatal("unauthenticated request reached application handler")
	}

	bootstrapURL := launch.URL() + "&view=first"
	response = mustRequest(t, client, http.MethodGet, bootstrapURL, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		t.Fatalf("unexpected launch cookie: %#v", cookies)
	}
	closeBody(t, response)
	if receivedQuery != "view=first" {
		t.Fatalf("application received query %q; bootstrap token was not stripped", receivedQuery)
	}
	if receivedRequestURI != "/?view=first" {
		t.Fatalf("application received RequestURI %q; bootstrap token was not stripped", receivedRequestURI)
	}

	cookieClient := &http.Client{Jar: newCookieJar(t)}
	bootstrap := mustRequest(t, cookieClient, http.MethodGet, launch.URL(), nil)
	closeBody(t, bootstrap)
	response = mustRequest(t, cookieClient, http.MethodGet, launch.BaseURL()+"dashboard", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("cookie-authenticated status = %d", response.StatusCode)
	}
	closeBody(t, response)

	header := http.Header{TokenHeader: []string{tokenFromLaunchURL(t, launch.URL())}}
	response = mustRequest(t, client, http.MethodGet, launch.BaseURL()+"header", header)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("header-authenticated status = %d", response.StatusCode)
	}
	closeBody(t, response)

	unsafe := http.Header{
		TokenHeader: []string{tokenFromLaunchURL(t, launch.URL())},
		"Origin":    []string{"https://example.invalid"},
	}
	response = mustRequest(t, client, http.MethodPost, launch.BaseURL()+"unsafe", unsafe)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin unsafe status = %d, want 403", response.StatusCode)
	}
	closeBody(t, response)

	health := mustRequest(t, client, http.MethodGet, launch.BaseURL()+"_singleserve/health", header)
	if health.StatusCode != http.StatusOK || health.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("health response = %d, cache %q", health.StatusCode, health.Header.Get("Cache-Control"))
	}
	closeBody(t, health)
	missing := mustRequest(t, client, http.MethodGet, launch.BaseURL()+"_singleserve/missing", header)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("reserved missing status = %d, want 404", missing.StatusCode)
	}
	closeBody(t, missing)
}

func TestApplicationHandlerDoesNotReceiveSingleserveCredentials(t *testing.T) {
	received := make(chan *http.Request, 1)
	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Clone(r.Context())
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
	token := tokenFromLaunchURL(t, launch.URL())
	request, err := http.NewRequest(http.MethodGet, launch.BaseURL()+"private?singleserve_token=decoy&view=one", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(TokenHeader, token)
	request.Header.Set("Authorization", "Bearer "+token)
	request.AddCookie(&http.Cookie{Name: server.cookie, Value: token})
	request.AddCookie(&http.Cookie{Name: "application", Value: "keep"})
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	closeBody(t, response)
	applicationRequest := <-received
	if applicationRequest.Header.Get(TokenHeader) != "" || applicationRequest.Header.Get("Authorization") != "" {
		t.Fatalf("application received Singleserve headers: %#v", applicationRequest.Header)
	}
	if strings.Contains(applicationRequest.Header.Get("Cookie"), server.cookie) || !strings.Contains(applicationRequest.Header.Get("Cookie"), "application=keep") {
		t.Fatalf("application cookies = %q", applicationRequest.Header.Get("Cookie"))
	}
	if applicationRequest.URL.Query().Has("singleserve_token") || strings.Contains(applicationRequest.RequestURI, "singleserve_token") {
		t.Fatalf("application URL = %q, RequestURI = %q", applicationRequest.URL.String(), applicationRequest.RequestURI)
	}

	request, err = http.NewRequest(http.MethodGet, launch.BaseURL()+"application-auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(TokenHeader, token)
	request.Header.Set("Authorization", "Basic application-credential")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	closeBody(t, response)
	if got := (<-received).Header.Get("Authorization"); got != "Basic application-credential" {
		t.Fatalf("application authorization = %q", got)
	}
}

func TestAuthenticationVariantsAndClientSource(t *testing.T) {
	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	token := tokenFromLaunchURL(t, launch.URL())

	bearer := http.Header{"Authorization": []string{"Bearer " + token}}
	response := mustRequest(t, http.DefaultClient, http.MethodGet, launch.BaseURL()+"bearer", bearer)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("bearer status = %d", response.StatusCode)
	}
	closeBody(t, response)

	unsafeQuery := launch.BaseURL() + "unsafe?singleserve_token=" + url.QueryEscape(token)
	response = mustRequest(t, http.DefaultClient, http.MethodPost, unsafeQuery, nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsafe query status = %d, want 401", response.StatusCode)
	}
	if strings.Contains(readBody(t, response), token) {
		t.Fatal("authentication failure echoed token")
	}

	preflight := http.Header{TokenHeader: []string{token}, "Origin": []string{"https://example.invalid"}}
	response = mustRequest(t, http.DefaultClient, http.MethodOptions, launch.BaseURL(), preflight)
	if response.StatusCode != http.StatusForbidden || response.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("preflight status = %d, CORS = %q", response.StatusCode, response.Header.Get("Access-Control-Allow-Origin"))
	}
	closeBody(t, response)

	clientResponse := mustRequest(t, http.DefaultClient, http.MethodGet, launch.BaseURL()+"_singleserve/client.js", http.Header{TokenHeader: []string{token}})
	if clientResponse.StatusCode != http.StatusOK || clientResponse.Header.Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("client response = %d, %q", clientResponse.StatusCode, clientResponse.Header.Get("Content-Type"))
	}
	canonical, err := os.ReadFile("client/singleserve.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBody(t, clientResponse); got != string(canonical) || got != string(browserClient) {
		t.Fatal("served browser client differs from embedded canonical source")
	}
}

func TestLaunchCookiesDoNotCollideAcrossPorts(t *testing.T) {
	first, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "first") })})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "second") })})
	if err != nil {
		t.Fatal(err)
	}
	firstLaunch, err := first.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	secondLaunch, err := second.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstLaunch.Shutdown(context.Background())
		_ = secondLaunch.Shutdown(context.Background())
	})
	client := &http.Client{Jar: newCookieJar(t)}
	closeBody(t, mustRequest(t, client, http.MethodGet, firstLaunch.URL(), nil))
	closeBody(t, mustRequest(t, client, http.MethodGet, secondLaunch.URL(), nil))
	firstResponse := mustRequest(t, client, http.MethodGet, firstLaunch.BaseURL(), nil)
	secondResponse := mustRequest(t, client, http.MethodGet, secondLaunch.BaseURL(), nil)
	if got := readBody(t, firstResponse); got != "first" {
		t.Fatalf("first app response = %q", got)
	}
	if got := readBody(t, secondResponse); got != "second" {
		t.Fatalf("second app response = %q", got)
	}
}

func TestTokenHas256BitsAndCookieIsLaunchUnique(t *testing.T) {
	first, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first.token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("token decoded length = %d, err = %v", len(decoded), err)
	}
	if first.token == second.token || first.cookie == second.cookie {
		t.Fatal("launch credentials were reused")
	}
}
