package singleserve

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

type authenticationMode uint8

const (
	authenticationNone authenticationMode = iota
	authenticationProgrammatic
	authenticationBrowserSession
)

func (s *Server) authenticatedHandler(baseURL string) http.Handler {
	expectedHost := strings.TrimSuffix(strings.TrimPrefix(baseURL, "http://"), "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sameLaunchHost(r, expectedHost) {
			writeControlError(w, http.StatusMisdirectedRequest, "host_forbidden", "Request host is not allowed")
			return
		}
		if s.handleBootstrap(w, r, baseURL) {
			return
		}

		mode := s.authenticationMode(r)
		if mode == authenticationNone {
			writeControlError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		if !originAllowed(r, baseURL, mode) {
			writeControlError(w, http.StatusForbidden, "origin_forbidden", "Request origin is not allowed")
			return
		}
		if r.URL.Path == strings.TrimSuffix(ControlPath, "/") || strings.HasPrefix(r.URL.Path, ControlPath) {
			s.handleControl(w, r)
			return
		}
		s.handler.ServeHTTP(w, s.stripAuthentication(r))
	})
}

func (s *Server) authenticationMode(r *http.Request) authenticationMode {
	if s == nil || r == nil {
		return authenticationNone
	}
	programCredentials := r.Header.Values(TokenHeader)
	if len(programCredentials) > 0 {
		if len(programCredentials) == 1 && tokenMatches(programCredentials[0], s.programToken) {
			return authenticationProgrammatic
		}
		return authenticationNone
	}
	if cookies := r.CookiesNamed(s.cookie); len(cookies) == 1 && tokenMatches(cookies[0].Value, s.sessionToken) {
		return authenticationBrowserSession
	}
	return authenticationNone
}

func (s *Server) hasBrowserSession(r *http.Request) bool {
	if s == nil || r == nil {
		return false
	}
	cookies := r.CookiesNamed(s.cookie)
	return len(cookies) == 1 && tokenMatches(cookies[0].Value, s.sessionToken)
}

func (s *Server) consumeBootstrap(candidate string) bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.bootstrapConsumed || s.bootstrapExpiresAt.IsZero() || !s.clock.Now().Before(s.bootstrapExpiresAt) {
		return false
	}
	if !tokenMatches(candidate, s.bootstrapToken) {
		return false
	}
	s.bootstrapConsumed = true
	return true
}

func tokenMatches(candidate, token string) bool {
	if candidate == "" || token == "" || len(candidate) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}

func (s *Server) setLaunchCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie,
		Value:    s.sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
	})
}

func (s *Server) stripAuthentication(r *http.Request) *http.Request {
	if r == nil {
		return r
	}
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Del(TokenHeader)
	clone.Header.Del(bootstrapHeader)
	clone.Header.Del(TabHeader)
	clone.Header.Del("Cookie")
	for _, cookie := range r.Cookies() {
		if cookie.Name != s.cookie {
			clone.AddCookie(cookie)
		}
	}
	if r.URL != nil {
		clone.URL = cloneURL(r.URL)
		query := clone.URL.Query()
		query.Del("singleserve_token")
		query.Del("singleserve_bootstrap")
		clone.URL.RawQuery = query.Encode()
		if clone.RequestURI != "" {
			clone.RequestURI = clone.URL.RequestURI()
		}
	}
	return clone
}

func cloneURL(source *url.URL) *url.URL {
	clone := *source
	return &clone
}

func sameLaunchHost(r *http.Request, expectedHost string) bool {
	return r != nil && r.Host != "" && strings.EqualFold(r.Host, expectedHost)
}

func originAllowed(r *http.Request, baseURL string, mode authenticationMode) bool {
	if r == nil {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return mode == authenticationProgrammatic || r.Method == http.MethodGet || r.Method == http.MethodHead
	}
	if len(origins) != 1 || origins[0] == "" {
		return false
	}
	return origins[0] == strings.TrimSuffix(baseURL, "/")
}
