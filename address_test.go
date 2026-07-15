package singleserve

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "default", want: "127.0.0.1:0"},
		{name: "empty host", input: ":8080", want: "127.0.0.1:8080"},
		{name: "unsupported IPv4 loopback alias", input: "127.0.0.2:90", wantErr: "loopback"},
		{name: "IPv6", input: "[::1]:0", want: "[::1]:0"},
		{name: "localhost", input: "localhost:0", want: "localhost:0"},
		{name: "wildcard IPv4", input: "0.0.0.0:0", wantErr: "loopback"},
		{name: "wildcard IPv6", input: "[::]:0", wantErr: "loopback"},
		{name: "hostname", input: "example.com:0", wantErr: "loopback"},
		{name: "missing port", input: "127.0.0.1", wantErr: "invalid address"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeAddress(test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("normalizeAddress(%q) error = %v, want %q", test.input, err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("normalizeAddress(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
}

func TestSupportedBindAddressesUseIsolatedLaunchHost(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "[::1]:0", "localhost:0"} {
		t.Run(address, func(t *testing.T) {
			server, err := New(Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}), Address: address})
			if err != nil {
				t.Fatal(err)
			}
			launch, err := server.Start(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
			base, err := url.Parse(launch.BaseURL())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(base.Hostname(), "ss-") || !strings.HasSuffix(base.Hostname(), ".localhost") {
				t.Fatalf("launch host = %q", base.Hostname())
			}
			listenerHost, _, err := net.SplitHostPort(launch.Address())
			if err != nil || !isLoopbackHost(strings.Trim(listenerHost, "[]")) {
				t.Fatalf("listener address = %q, err = %v", launch.Address(), err)
			}
			response, err := launch.Client().Get(launch.BaseURL())
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				t.Fatalf("status = %d", response.StatusCode)
			}
		})
	}
}
