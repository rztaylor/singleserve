package singleserve

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) authenticatedHandler(baseURL string) http.Handler {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == strings.TrimSuffix(ControlPath, "/") || strings.HasPrefix(r.URL.Path, ControlPath) {
			s.handleControl(w, r)
			return
		}
		s.handler.ServeHTTP(w, r)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sameOriginForUnsafeRequest(r, baseURL) {
			writeControlError(w, http.StatusForbidden, "origin_forbidden", "Request origin is not allowed")
			return
		}

		authorized, bootstrap := s.authorized(r)
		if !authorized {
			writeControlError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		if bootstrap {
			s.setLaunchCookie(w)
			w.Header().Set("Referrer-Policy", "no-referrer")
		}
		next.ServeHTTP(w, s.stripAuthentication(r))
	})
}

func (s *Server) authorized(r *http.Request) (bool, bool) {
	if s == nil || s.token == "" {
		return false, false
	}
	if tokenMatches(r.Header.Get(TokenHeader), s.token) {
		return true, false
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); len(auth) > len("Bearer ") && strings.EqualFold(auth[:len("Bearer ")], "Bearer ") && tokenMatches(strings.TrimSpace(auth[len("Bearer "):]), s.token) {
		return true, false
	}
	if cookie, err := r.Cookie(s.cookie); err == nil && tokenMatches(cookie.Value, s.token) {
		return true, false
	}
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && tokenMatches(r.URL.Query().Get("singleserve_token"), s.token) {
		return true, true
	}
	return false, false
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
		Value:    s.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	})
}

func (s *Server) stripAuthentication(r *http.Request) *http.Request {
	if r == nil {
		return r
	}
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Del(TokenHeader)
	if auth := strings.TrimSpace(clone.Header.Get("Authorization")); len(auth) > len("Bearer ") && strings.EqualFold(auth[:len("Bearer ")], "Bearer ") && tokenMatches(strings.TrimSpace(auth[len("Bearer "):]), s.token) {
		clone.Header.Del("Authorization")
	}
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

func sameOriginForUnsafeRequest(r *http.Request, baseURL string) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, base.Scheme) && strings.EqualFold(parsed.Host, base.Host) && (parsed.Path == "" || parsed.Path == "/")
}
