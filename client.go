package singleserve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type launchTransport struct {
	base   http.RoundTripper
	origin string
	token  string
}

func (t *launchTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, fmt.Errorf("singleserve: programmatic request is nil")
	}
	if !sameURLOrigin(request.URL, t.origin) {
		return nil, fmt.Errorf("singleserve: refusing to send authentication outside the launch origin")
	}
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	if clone.Header == nil {
		clone.Header = make(http.Header)
	}
	clone.Header.Set(TokenHeader, t.token)
	return t.base.RoundTrip(clone)
}

func (t *launchTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// Client returns an HTTP client authenticated only for this launch origin.
// The client bypasses environment proxies, dials the bound listener directly,
// and never exposes its credential.
func (l *Launch) Client() *http.Client {
	if l == nil || l.server == nil {
		return nil
	}
	l.server.mu.Lock()
	origin := strings.TrimSuffix(l.server.baseURL, "/")
	address := l.server.httpServer.Addr
	token := l.server.programToken
	l.server.mu.Unlock()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}
	return &http.Client{
		Transport: &launchTransport{base: transport, origin: origin, token: token},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("singleserve: stopped after 10 redirects")
			}
			if request == nil || request.URL == nil || !sameURLOrigin(request.URL, origin) {
				return fmt.Errorf("singleserve: refusing redirect outside the launch origin")
			}
			return nil
		},
	}
}

func sameURLOrigin(target *url.URL, origin string) bool {
	if target == nil || target.Scheme == "" || target.Host == "" {
		return false
	}
	return strings.EqualFold(target.Scheme+"://"+target.Host, origin)
}
