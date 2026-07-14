package singleserve_test

import (
	"encoding/base64"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	"github.com/rztaylor/singleserve"
)

const consumerHarnessTimeout = time.Second

func browserClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: consumerHarnessTimeout}
}

func bootstrapConsumer(t *testing.T, client *http.Client, launchURL string) (baseURL, token string) {
	t.Helper()
	parsed, err := url.Parse(launchURL)
	if err != nil {
		t.Fatal(err)
	}
	token = parsed.Query().Get("singleserve_token")
	if token == "" {
		t.Fatal("launch URL did not include an authentication token")
	}
	response, err := client.Get(launchURL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d", response.StatusCode)
	}
	parsed.RawQuery = ""
	baseURL = (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
	return baseURL, token
}

func tabID(seed byte) string {
	return base64.RawURLEncoding.EncodeToString([]byte{
		seed, seed, seed, seed, seed, seed, seed, seed,
		seed, seed, seed, seed, seed, seed, seed, seed,
	})
}

func controlRequest(t *testing.T, client *http.Client, method, rawURL, token, tab string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(singleserve.TokenHeader, token)
	request.Header.Set(singleserve.TabHeader, tab)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String())
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
