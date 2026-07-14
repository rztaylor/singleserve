package singleserve

import (
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
		{name: "IPv4", input: "127.0.0.2:90", want: "127.0.0.2:90"},
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
