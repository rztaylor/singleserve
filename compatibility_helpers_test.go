package singleserve_test

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/rztaylor/singleserve"
	"github.com/rztaylor/singleserve/internal/testbrowserjar"
)

const consumerHarnessTimeout = time.Second

func browserClient(t *testing.T) *http.Client {
	t.Helper()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Jar: testbrowserjar.New(), Timeout: consumerHarnessTimeout, Transport: transport}
}

func bootstrapConsumer(t *testing.T, client *http.Client, launchURL string) (baseURL string) {
	t.Helper()
	parsed, err := url.Parse(launchURL)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Fragment
	if token == "" {
		t.Fatal("launch URL did not include a bootstrap capability")
	}
	response, err := client.Get(launchURL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d", response.StatusCode)
	}
	parsed.Fragment = ""
	baseURL = (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
	request, err := http.NewRequest(http.MethodPost, baseURL+"/_singleserve/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Singleserve-Bootstrap", token)
	request.Header.Set("Origin", baseURL)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap exchange status = %d", response.StatusCode)
	}
	return baseURL
}

func tabID(seed byte) string {
	return base64.RawURLEncoding.EncodeToString([]byte{
		seed, seed, seed, seed, seed, seed, seed, seed,
		seed, seed, seed, seed, seed, seed, seed, seed,
	})
}

func controlRequest(t *testing.T, client *http.Client, method, rawURL, tab string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
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
