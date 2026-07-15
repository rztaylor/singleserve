package singleserve

import (
	"fmt"
	"net"
	"strings"
)

const defaultAddress = "127.0.0.1:0"

func normalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return defaultAddress, nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if strings.HasPrefix(address, ":") {
			return "127.0.0.1" + address, nil
		}
		return "", fmt.Errorf("singleserve: invalid address %q: %w", address, err)
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		host = "127.0.0.1"
	}
	if !isLoopbackHost(host) {
		return "", fmt.Errorf("singleserve: server must bind to loopback, got %q", host)
	}
	return net.JoinHostPort(host, port), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.Equal(net.IPv4(127, 0, 0, 1)) || ip.Equal(net.IPv6loopback))
}
