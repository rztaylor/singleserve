package singleserve

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerOwnedBootstrapAndBrowserSession(t *testing.T) {
	var appRequests atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appRequests.Add(1)
		if r.URL.RawQuery != "view=first" || r.RequestURI != "/dashboard?view=first" {
			t.Errorf("application URL = %q, RequestURI = %q", r.URL.String(), r.RequestURI)
		}
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

	parsed, err := url.Parse(launch.URL())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.RawQuery != "" || parsed.Fragment == "" || parsed.Path != ControlPath+"bootstrap" || !strings.HasPrefix(parsed.Hostname(), "ss-") || !strings.HasSuffix(parsed.Hostname(), ".localhost") {
		t.Fatalf("launch URL = %q", launch.URL())
	}
	if strings.Contains(parsed.RequestURI(), parsed.Fragment) {
		t.Fatal("bootstrap capability entered HTTP request URI")
	}

	client := newDirectClient(t, true)
	unauthenticated := mustRequest(t, client, http.MethodGet, launch.BaseURL()+"dashboard", nil)
	if unauthenticated.StatusCode != http.StatusUnauthorized || appRequests.Load() != 0 {
		t.Fatalf("unauthenticated response = %d, app requests = %d", unauthenticated.StatusCode, appRequests.Load())
	}
	closeBody(t, unauthenticated)

	page := mustRequest(t, client, http.MethodGet, launch.URL(), nil)
	if page.StatusCode != http.StatusOK || page.Header.Get("Referrer-Policy") != "no-referrer" || page.Header.Get("Cache-Control") != "no-store" || page.Header.Get("X-Content-Type-Options") != "nosniff" || page.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("bootstrap response = %d, headers %#v", page.StatusCode, page.Header)
	}
	if len(page.Cookies()) != 0 || appRequests.Load() != 0 {
		t.Fatal("bootstrap page set a cookie or reached the application")
	}
	closeBody(t, page)

	headers := http.Header{
		bootstrapHeader: []string{parsed.Fragment},
		"Origin":        []string{strings.TrimSuffix(launch.BaseURL(), "/")},
	}
	exchange := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", headers)
	if exchange.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap exchange status = %d, body = %s", exchange.StatusCode, readBody(t, exchange))
	}
	cookies := exchange.Cookies()
	if len(cookies) != 1 || cookies[0].Value == parsed.Fragment || !strings.HasPrefix(cookies[0].Name, "__Host-") || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" || !cookies[0].Secure || cookies[0].Domain != "" || cookies[0].MaxAge != 0 {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}
	closeBody(t, exchange)

	response := mustRequest(t, client, http.MethodGet, launch.BaseURL()+"dashboard?view=first", nil)
	if response.StatusCode != http.StatusOK || readBody(t, response) != "application" {
		t.Fatalf("cookie-authenticated status = %d", response.StatusCode)
	}

	replayClient := newDirectClient(t, true)
	replay := mustRequest(t, replayClient, http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", headers)
	if replay.StatusCode != http.StatusUnauthorized || strings.Contains(readBody(t, replay), parsed.Fragment) {
		t.Fatalf("bootstrap replay status = %d", replay.StatusCode)
	}
	recovery := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", headers)
	if recovery.StatusCode != http.StatusOK {
		t.Fatalf("same-session bootstrap recovery status = %d", recovery.StatusCode)
	}
	closeBody(t, recovery)
}

func TestAuthenticationProvenanceAndCredentialStripping(t *testing.T) {
	received := make(chan *http.Request, 4)
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

	programmatic := launch.Client()
	request, err := http.NewRequest(http.MethodGet, launch.BaseURL()+"private?singleserve_token=decoy&singleserve_bootstrap=decoy&view=one", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(TokenHeader, "caller-supplied-decoy")
	request.Header.Set(bootstrapHeader, "caller-bootstrap-decoy")
	request.Header.Set(TabHeader, testTabID(3))
	request.Header.Set("Authorization", "Basic application-credential")
	request.AddCookie(&http.Cookie{Name: server.cookie, Value: server.sessionToken})
	request.AddCookie(&http.Cookie{Name: "application", Value: "keep"})
	response, err := programmatic.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	closeBody(t, response)
	applicationRequest := <-received
	if applicationRequest.Header.Get(TokenHeader) != "" || applicationRequest.Header.Get(bootstrapHeader) != "" || applicationRequest.Header.Get(TabHeader) != "" {
		t.Fatalf("application received Singleserve headers: %#v", applicationRequest.Header)
	}
	if applicationRequest.Header.Get("Authorization") != "Basic application-credential" {
		t.Fatalf("application authorization = %q", applicationRequest.Header.Get("Authorization"))
	}
	if strings.Contains(applicationRequest.Header.Get("Cookie"), server.cookie) || !strings.Contains(applicationRequest.Header.Get("Cookie"), "application=keep") {
		t.Fatalf("application cookies = %q", applicationRequest.Header.Get("Cookie"))
	}
	if applicationRequest.URL.Query().Has("singleserve_token") || applicationRequest.URL.Query().Has("singleserve_bootstrap") || strings.Contains(applicationRequest.RequestURI, "singleserve_") {
		t.Fatalf("application URL = %q, RequestURI = %q", applicationRequest.URL.String(), applicationRequest.RequestURI)
	}

	wrongOrigin := http.Header{"Origin": []string{"https://example.invalid"}}
	response = mustRequest(t, programmatic, http.MethodPost, launch.BaseURL()+"unsafe", wrongOrigin)
	if response.StatusCode != http.StatusForbidden || response.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("programmatic cross-origin status = %d", response.StatusCode)
	}
	closeBody(t, response)

	browser := newDirectClient(t, true)
	bootstrapBrowser(t, launch, browser)
	response = mustRequest(t, browser, http.MethodPost, launch.BaseURL()+"unsafe", nil)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cookie request without Origin status = %d", response.StatusCode)
	}
	closeBody(t, response)
	exactOrigin := http.Header{"Origin": []string{strings.TrimSuffix(launch.BaseURL(), "/")}}
	response = mustRequest(t, browser, http.MethodPost, launch.BaseURL()+"unsafe", exactOrigin)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("cookie exact-Origin status = %d", response.StatusCode)
	}
	closeBody(t, response)
}

func TestExactHostAndRemovedAuthenticationVariants(t *testing.T) {
	var appRequests atomic.Int32
	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		appRequests.Add(1)
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
	client := newDirectClient(t, false)

	request, err := http.NewRequest(http.MethodGet, launch.BaseURL(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "localhost:" + request.URL.Port()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusMisdirectedRequest || appRequests.Load() != 0 {
		t.Fatalf("wrong Host response = %d, app requests = %d", response.StatusCode, appRequests.Load())
	}
	closeBody(t, response)

	bootstrap := bootstrapTokenFromLaunchURL(t, launch.URL())
	headers := http.Header{"Authorization": []string{"Bearer " + bootstrap}}
	response = mustRequest(t, client, http.MethodGet, launch.BaseURL(), headers)
	if response.StatusCode != http.StatusUnauthorized || strings.Contains(readBody(t, response), bootstrap) {
		t.Fatalf("removed bearer status = %d", response.StatusCode)
	}
	unsafeQuery := launch.BaseURL() + "unsafe?singleserve_token=" + url.QueryEscape(bootstrap)
	response = mustRequest(t, client, http.MethodPost, unsafeQuery, nil)
	if response.StatusCode != http.StatusUnauthorized || strings.Contains(readBody(t, response), bootstrap) {
		t.Fatalf("removed query status = %d", response.StatusCode)
	}
}

func TestCanonicalClientsAndProgrammaticOriginGuard(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	client := launch.Client()

	transport, ok := client.Transport.(*launchTransport)
	if !ok {
		t.Fatalf("programmatic transport = %T", client.Transport)
	}
	base, ok := transport.base.(*http.Transport)
	if !ok || base.Proxy != nil {
		t.Fatal("programmatic client did not disable environment proxies")
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.invalid/collect", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(request); err == nil || !strings.Contains(err.Error(), "outside the launch origin") {
		t.Fatalf("cross-origin programmatic error = %v", err)
	}
	parsedBase, err := url.Parse(launch.BaseURL())
	if err != nil {
		t.Fatal(err)
	}
	nilHeaderResponse, err := client.Do(&http.Request{Method: http.MethodGet, URL: parsedBase})
	if err != nil {
		t.Fatal(err)
	}
	closeBody(t, nilHeaderResponse)

	clientResponse := mustRequest(t, client, http.MethodGet, launch.BaseURL()+"_singleserve/client.js", nil)
	if clientResponse.StatusCode != http.StatusOK || clientResponse.Header.Get("Content-Type") != "text/javascript; charset=utf-8" || clientResponse.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("client response = %d, %q", clientResponse.StatusCode, clientResponse.Header.Get("Content-Type"))
	}
	canonical, err := os.ReadFile("client/singleserve.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBody(t, clientResponse); got != string(canonical) || got != string(browserClient) {
		t.Fatal("served browser client differs from embedded canonical source")
	}

	bootstrapResponse := mustRequest(t, newDirectClient(t, false), http.MethodGet, launch.BaseURL()+"_singleserve/bootstrap.js", nil)
	bootstrapCanonical, err := os.ReadFile("client/bootstrap.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBody(t, bootstrapResponse); got != string(bootstrapCanonical) || got != string(bootstrapClient) {
		t.Fatal("served bootstrap client differs from canonical source")
	}
}

func TestLaunchCookiesAreIsolatedByHost(t *testing.T) {
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
	client := newDirectClient(t, true)
	bootstrapBrowser(t, firstLaunch, client)

	seenCookies := make(chan string, 1)
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookies <- r.Header.Get("Cookie")
		http.SetCookie(w, &http.Cookie{Name: first.cookie, Value: "hostile", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hostile.Close()
	hostileResponse := mustRequest(t, client, http.MethodGet, hostile.URL, nil)
	closeBody(t, hostileResponse)
	if got := <-seenCookies; got != "" {
		t.Fatalf("sibling loopback service received cookie %q", got)
	}

	bootstrapBrowser(t, secondLaunch, client)
	firstResponse := mustRequest(t, client, http.MethodGet, firstLaunch.BaseURL(), nil)
	secondResponse := mustRequest(t, client, http.MethodGet, secondLaunch.BaseURL(), nil)
	if got := readBody(t, firstResponse); got != "first" {
		t.Fatalf("first app response = %q", got)
	}
	if got := readBody(t, secondResponse); got != "second" {
		t.Fatalf("second app response = %q", got)
	}
}

func TestAuthenticationMaterialIsIndependentAndStrong(t *testing.T) {
	first, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	values := []string{first.bootstrapToken, first.sessionToken, first.programToken}
	seen := make(map[string]bool)
	for _, value := range values {
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(value)
		if decodeErr != nil || len(decoded) != 32 || seen[value] {
			t.Fatalf("credential length = %d, err = %v, duplicate = %v", len(decoded), decodeErr, seen[value])
		}
		seen[value] = true
	}
	originLabel := strings.TrimSuffix(strings.TrimPrefix(first.originHost, "ss-"), ".localhost")
	decodedOrigin, err := hex.DecodeString(originLabel)
	if err != nil || len(decodedOrigin) != 16 {
		t.Fatalf("origin host = %q, decoded length = %d, err = %v", first.originHost, len(decodedOrigin), err)
	}
	if first.cookie == second.cookie || first.originHost == second.originHost || first.bootstrapToken == second.bootstrapToken || first.sessionToken == second.sessionToken || first.programToken == second.programToken {
		t.Fatal("launch authentication material was reused")
	}
}

func TestBootstrapExpiresAndIsConsumedAtomically(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	server, err := newServer(Options{Handler: http.NotFoundHandler()}, clock, nil, strings.NewReader(strings.Repeat("e", 112)))
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	clock.advance(now.Add(defaultBootstrapTimeout))
	expiredHeaders := http.Header{
		bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, launch.URL())},
		"Origin":        []string{strings.TrimSuffix(launch.BaseURL(), "/")},
	}
	expired := mustRequest(t, newDirectClient(t, false), http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", expiredHeaders)
	if expired.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired bootstrap status = %d", expired.StatusCode)
	}
	closeBody(t, expired)

	concurrentServer, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	concurrentLaunch, err := concurrentServer.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = concurrentLaunch.Shutdown(context.Background()) })
	headers := http.Header{
		bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, concurrentLaunch.URL())},
		"Origin":        []string{strings.TrimSuffix(concurrentLaunch.BaseURL(), "/")},
	}

	const attempts = 16
	statuses := make(chan int, attempts)
	errors := make(chan error, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			request, requestErr := http.NewRequest(http.MethodPost, concurrentLaunch.BaseURL()+"_singleserve/bootstrap", nil)
			if requestErr != nil {
				errors <- requestErr
				return
			}
			request.Header = headers.Clone()
			response, requestErr := newDirectClient(t, false).Do(request)
			if requestErr != nil {
				errors <- requestErr
				return
			}
			_ = response.Body.Close()
			statuses <- response.StatusCode
		}()
	}
	group.Wait()
	close(statuses)
	close(errors)
	for requestErr := range errors {
		t.Errorf("concurrent bootstrap: %v", requestErr)
	}
	successes := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			successes++
		case http.StatusUnauthorized:
		default:
			t.Errorf("concurrent bootstrap status = %d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent bootstrap exchanges = %d", successes)
	}
}

func TestNewBootstrapURLRotatesAndResetsExpiry(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	entropy := strings.NewReader(
		strings.Repeat("a", 32) +
			strings.Repeat("b", 32) +
			strings.Repeat("c", 32) +
			strings.Repeat("d", 16) +
			strings.Repeat("e", 32) +
			strings.Repeat("f", 32) +
			strings.Repeat("g", 32),
	)
	server, err := newServer(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}, clock, nil, entropy)
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	initialURL := launch.URL()
	clock.advance(now.Add(30 * time.Second))
	renewedURL, err := launch.NewBootstrapURL()
	if err != nil {
		t.Fatal(err)
	}
	if renewedURL == initialURL || launch.URL() != renewedURL {
		t.Fatalf("initial URL = %q, renewed URL = %q, current URL = %q", initialURL, renewedURL, launch.URL())
	}
	initial, err := url.Parse(initialURL)
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := url.Parse(renewedURL)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Scheme != renewed.Scheme || initial.Host != renewed.Host || initial.Path != renewed.Path || initial.Fragment == renewed.Fragment {
		t.Fatalf("initial URL = %q, renewed URL = %q", initialURL, renewedURL)
	}

	origin := strings.TrimSuffix(launch.BaseURL(), "/")
	oldResponse := mustRequest(t, newDirectClient(t, false), http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", http.Header{
		bootstrapHeader: []string{initial.Fragment},
		"Origin":        []string{origin},
	})
	if oldResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalidated bootstrap status = %d", oldResponse.StatusCode)
	}
	closeBody(t, oldResponse)

	clock.advance(now.Add(30*time.Second + defaultBootstrapTimeout))
	expiredResponse := mustRequest(t, newDirectClient(t, false), http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", http.Header{
		bootstrapHeader: []string{renewed.Fragment},
		"Origin":        []string{origin},
	})
	if expiredResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("renewed bootstrap at deadline status = %d", expiredResponse.StatusCode)
	}
	closeBody(t, expiredResponse)

	secondURL, err := launch.NewBootstrapURL()
	if err != nil {
		t.Fatal(err)
	}
	browser := newDirectClient(t, true)
	secondResponse := mustRequest(t, browser, http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", http.Header{
		bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, secondURL)},
		"Origin":        []string{origin},
	})
	if secondResponse.StatusCode != http.StatusOK {
		t.Fatalf("second renewed bootstrap status = %d", secondResponse.StatusCode)
	}
	closeBody(t, secondResponse)

	if _, err := launch.NewBootstrapURL(); err != nil {
		t.Fatal(err)
	}
	applicationResponse := mustRequest(t, browser, http.MethodGet, launch.BaseURL(), nil)
	if applicationResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("established session after renewal status = %d", applicationResponse.StatusCode)
	}
	closeBody(t, applicationResponse)
}

func TestNewBootstrapURLGenerationFailurePreservesCurrentCapability(t *testing.T) {
	server, err := newServer(Options{Handler: http.NotFoundHandler()}, realClock{}, nil, strings.NewReader(strings.Repeat("x", 112)))
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	initialURL := launch.URL()
	if _, err := launch.NewBootstrapURL(); err == nil || !strings.Contains(err.Error(), "generate bootstrap credential") {
		t.Fatalf("renewal error = %v", err)
	}
	if launch.URL() != initialURL {
		t.Fatalf("URL changed after failed renewal: got %q, want %q", launch.URL(), initialURL)
	}
	response := mustRequest(t, newDirectClient(t, false), http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", http.Header{
		bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, initialURL)},
		"Origin":        []string{strings.TrimSuffix(launch.BaseURL(), "/")},
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("preserved bootstrap status = %d", response.StatusCode)
	}
	closeBody(t, response)
}

func TestConcurrentNewBootstrapURLKeepsOneCapabilityActive(t *testing.T) {
	server, err := New(Options{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })

	const renewals = 16
	initialURL := launch.URL()
	urls := make(chan string, renewals)
	errors := make(chan error, renewals)
	var group sync.WaitGroup
	for range renewals {
		group.Add(1)
		go func() {
			defer group.Done()
			rawURL, renewalErr := launch.NewBootstrapURL()
			if renewalErr != nil {
				errors <- renewalErr
				return
			}
			urls <- rawURL
		}()
	}
	group.Wait()
	close(urls)
	close(errors)
	for renewalErr := range errors {
		t.Errorf("concurrent renewal: %v", renewalErr)
	}

	issued := []string{initialURL}
	seen := make(map[string]bool)
	for rawURL := range urls {
		if seen[rawURL] {
			t.Fatalf("duplicate renewed URL %q", rawURL)
		}
		seen[rawURL] = true
		issued = append(issued, rawURL)
	}
	if len(seen) != renewals || !seen[launch.URL()] {
		t.Fatalf("issued %d unique URLs; current URL was issued = %v", len(seen), seen[launch.URL()])
	}

	successes := 0
	for _, rawURL := range issued {
		response := mustRequest(t, newDirectClient(t, false), http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", http.Header{
			bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, rawURL)},
			"Origin":        []string{strings.TrimSuffix(launch.BaseURL(), "/")},
		})
		if response.StatusCode == http.StatusOK {
			successes++
		} else if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("concurrent renewed bootstrap status = %d", response.StatusCode)
		}
		closeBody(t, response)
	}
	if successes != 1 {
		t.Fatalf("active concurrent renewed capabilities = %d", successes)
	}
}

func TestAmbiguousAuthenticationAndOriginsFailClosed(t *testing.T) {
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
	origin := strings.TrimSuffix(launch.BaseURL(), "/")

	tests := []struct {
		name    string
		origins []string
	}{
		{name: "duplicate", origins: []string{origin, origin}},
		{name: "empty", origins: []string{""}},
		{name: "opaque", origins: []string{"null"}},
		{name: "trailing slash", origins: []string{origin + "/"}},
		{name: "cross port", origins: []string{origin + "0"}},
		{name: "comma joined", origins: []string{origin + ", " + origin}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, requestErr := http.NewRequest(http.MethodPost, launch.BaseURL()+"unsafe", nil)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			request.Header["Origin"] = test.origins
			response, requestErr := launch.Client().Do(request)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d", response.StatusCode)
			}
			closeBody(t, response)
		})
	}

	direct := newDirectClient(t, false)
	duplicateToken, err := http.NewRequest(http.MethodGet, launch.BaseURL(), nil)
	if err != nil {
		t.Fatal(err)
	}
	duplicateToken.Header[TokenHeader] = []string{server.programToken, server.programToken}
	response, err := direct.Do(duplicateToken)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("duplicate programmatic credential status = %d", response.StatusCode)
	}
	closeBody(t, response)
	duplicateBootstrap, err := http.NewRequest(http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	duplicateBootstrap.Header[bootstrapHeader] = []string{server.bootstrapToken, server.bootstrapToken}
	duplicateBootstrap.Header.Set("Origin", origin)
	response, err = direct.Do(duplicateBootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("duplicate bootstrap credential status = %d", response.StatusCode)
	}
	closeBody(t, response)

	browser := newDirectClient(t, true)
	bootstrapBrowser(t, launch, browser)
	ambiguousCookie, err := http.NewRequest(http.MethodPost, launch.BaseURL()+"unsafe", nil)
	if err != nil {
		t.Fatal(err)
	}
	ambiguousCookie.Header.Set("Origin", origin)
	ambiguousCookie.AddCookie(&http.Cookie{Name: server.cookie, Value: server.sessionToken})
	response, err = browser.Do(ambiguousCookie)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("duplicate session cookie status = %d", response.StatusCode)
	}
	closeBody(t, response)

	invalidProgrammatic, err := http.NewRequest(http.MethodPost, launch.BaseURL()+"unsafe", nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidProgrammatic.Header.Set(TokenHeader, "invalid")
	invalidProgrammatic.Header.Set("Origin", origin)
	response, err = browser.Do(invalidProgrammatic)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid header with valid cookie status = %d", response.StatusCode)
	}
	closeBody(t, response)
}

func TestProgrammaticClientRejectsOffOriginRedirect(t *testing.T) {
	var hostileRequests atomic.Int32
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hostileRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hostile.Close()

	server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, hostile.URL, http.StatusFound)
	})})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	response, err := launch.Client().Get(launch.BaseURL())
	if err == nil || !strings.Contains(err.Error(), "outside the launch origin") {
		if response != nil {
			closeBody(t, response)
		}
		t.Fatalf("off-origin redirect error = %v", err)
	}
	if hostileRequests.Load() != 0 {
		t.Fatalf("hostile redirect requests = %d", hostileRequests.Load())
	}
}

func FuzzOriginAllowed(f *testing.F) {
	f.Add("")
	f.Add("null")
	f.Add("http://ss-example.localhost:1234")
	f.Add("http://ss-example.localhost:1234/")
	f.Fuzz(func(t *testing.T, origin string) {
		request := httptest.NewRequest(http.MethodPost, "http://ss-example.localhost:1234/unsafe", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		allowed := originAllowed(request, "http://ss-example.localhost:1234/", authenticationBrowserSession)
		if allowed != (origin == "http://ss-example.localhost:1234") {
			t.Fatalf("originAllowed(%q) = %v", origin, allowed)
		}
	})
}

func FuzzSameLaunchHost(f *testing.F) {
	const expected = "ss-0123456789abcdef0123456789abcdef.localhost:4321"
	f.Add(expected)
	f.Add(strings.ToUpper(expected))
	f.Add(expected + ".")
	f.Add("localhost:4321")
	f.Fuzz(func(t *testing.T, host string) {
		request := httptest.NewRequest(http.MethodGet, "http://"+expected+"/", nil)
		request.Host = host
		if got, want := sameLaunchHost(request, expected), strings.EqualFold(host, expected); got != want {
			t.Fatalf("sameLaunchHost(%q) = %v, want %v", host, got, want)
		}
	})
}
