package singleserve

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rztaylor/singleserve/internal/testbrowserjar"
)

func mustRequest(t *testing.T, client *http.Client, method, rawURL string, headers http.Header) *http.Response {
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

func bootstrapTokenFromLaunchURL(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Fragment
	if token == "" {
		t.Fatalf("launch URL %q has no bootstrap fragment", rawURL)
	}
	return token
}

func newDirectClient(t *testing.T, withJar bool) *http.Client {
	t.Helper()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport}
	if withJar {
		client.Jar = newCookieJar(t)
	}
	return client
}

func bootstrapBrowser(t *testing.T, launch *Launch, client *http.Client) {
	t.Helper()
	page := mustRequest(t, client, http.MethodGet, launch.URL(), nil)
	if page.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap page status = %d", page.StatusCode)
	}
	closeBody(t, page)
	headers := http.Header{
		bootstrapHeader: []string{bootstrapTokenFromLaunchURL(t, launch.URL())},
		"Origin":        []string{strings.TrimSuffix(launch.BaseURL(), "/")},
	}
	exchange := mustRequest(t, client, http.MethodPost, launch.BaseURL()+"_singleserve/bootstrap", headers)
	if exchange.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap exchange status = %d, body = %s", exchange.StatusCode, readBody(t, exchange))
	}
	closeBody(t, exchange)
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

func closeBody(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func newCookieJar(t *testing.T) http.CookieJar {
	t.Helper()
	return testbrowserjar.New()
}
