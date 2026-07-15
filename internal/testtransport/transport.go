// Package testtransport provides browser-like loopback name resolution for
// repository HTTP tests. It is test support, not production transport logic.
package testtransport

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// New returns a no-proxy transport that resolves localhost names directly to
// the IPv4 loopback address while preserving the request URL and Host header.
func New() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, loopbackAddress(address))
	}
	return transport
}

func loopbackAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	host = strings.ToLower(host)
	if host != "localhost" && !strings.HasSuffix(host, ".localhost") {
		return address
	}
	return net.JoinHostPort("127.0.0.1", port)
}
