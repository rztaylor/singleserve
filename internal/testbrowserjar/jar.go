// Package testbrowserjar provides browser-like host-only cookie behavior for
// repository HTTP tests. It is test support, not production authentication.
package testbrowserjar

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// New returns a test jar that accepts Secure __Host- cookies from trustworthy
// .localhost HTTP origins while keeping cookies isolated by exact host.
func New() http.CookieJar {
	return &jar{cookies: make(map[string]map[string]string)}
}

type jar struct {
	mu      sync.Mutex
	cookies map[string]map[string]string
}

func (j *jar) SetCookies(target *url.URL, cookies []*http.Cookie) {
	if target == nil {
		return
	}
	host := strings.ToLower(target.Hostname())
	if host == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" || cookie.Domain != "" {
			continue
		}
		if cookie.Secure && target.Scheme != "https" && host != "localhost" && !strings.HasSuffix(host, ".localhost") {
			continue
		}
		if strings.HasPrefix(cookie.Name, "__Host-") && (!cookie.Secure || cookie.Path != "/") {
			continue
		}
		if cookie.MaxAge < 0 {
			delete(j.cookies[host], cookie.Name)
			continue
		}
		if j.cookies[host] == nil {
			j.cookies[host] = make(map[string]string)
		}
		j.cookies[host][cookie.Name] = cookie.Value
	}
}

func (j *jar) Cookies(target *url.URL) []*http.Cookie {
	if target == nil {
		return nil
	}
	host := strings.ToLower(target.Hostname())
	j.mu.Lock()
	defer j.mu.Unlock()
	values := j.cookies[host]
	result := make([]*http.Cookie, 0, len(values))
	for name, value := range values {
		result = append(result, &http.Cookie{Name: name, Value: value})
	}
	return result
}
