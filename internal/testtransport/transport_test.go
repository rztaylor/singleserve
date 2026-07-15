package testtransport

import "testing"

func TestLoopbackAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{name: "isolated localhost", address: "ss-example.localhost:4321", want: "127.0.0.1:4321"},
		{name: "uppercase localhost", address: "SS-EXAMPLE.LOCALHOST:4321", want: "127.0.0.1:4321"},
		{name: "localhost", address: "localhost:4321", want: "127.0.0.1:4321"},
		{name: "unrelated host", address: "example.com:4321", want: "example.com:4321"},
		{name: "malformed address", address: "missing-port", want: "missing-port"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := loopbackAddress(test.address); got != test.want {
				t.Fatalf("loopbackAddress(%q) = %q, want %q", test.address, got, test.want)
			}
		})
	}
}
