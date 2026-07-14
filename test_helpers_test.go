package singleserve

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
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

func tokenFromLaunchURL(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Query().Get("singleserve_token")
	if token == "" {
		t.Fatalf("launch URL %q has no token", rawURL)
	}
	return token
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
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}
