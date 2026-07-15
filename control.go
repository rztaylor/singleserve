package singleserve

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// TokenHeader carries the private programmatic credential applied by
	// Launch.Client. Browser clients do not send it.
	TokenHeader = "X-Singleserve-Token"
	// TabHeader identifies one browser tab instance.
	TabHeader = "X-Singleserve-Tab"
	// ControlPath is reserved for Singleserve lifecycle endpoints.
	ControlPath     = "/_singleserve/"
	bootstrapHeader = "X-Singleserve-Bootstrap"
)

//go:embed client/singleserve.js
var browserClient []byte

//go:embed client/bootstrap.js
var bootstrapClient []byte

const bootstrapHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Starting local application</title>
</head>
<body>
<main>
<h1>Starting local application</h1>
<p id="status" role="status">Establishing a secure browser session…</p>
</main>
<script type="module" src="/_singleserve/bootstrap.js"></script>
</body>
</html>
`

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request, baseURL string) bool {
	if r.URL.Path != ControlPath+"bootstrap" && r.URL.Path != ControlPath+"bootstrap.js" {
		return false
	}
	setControlSecurityHeaders(w)
	w.Header().Set("Referrer-Policy", "no-referrer")
	if rejectRequestBody(w, r) {
		return true
	}
	if r.URL.Path == ControlPath+"bootstrap.js" {
		if !requireMethod(w, r, http.MethodGet) {
			return true
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bootstrapClient)
		return true
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(bootstrapHTML))
	case http.MethodPost:
		if !originAllowed(r, baseURL, authenticationBrowserSession) {
			writeControlError(w, http.StatusForbidden, "origin_forbidden", "Request origin is not allowed")
			return true
		}
		if s.hasBrowserSession(r) {
			writeControlJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return true
		}
		bootstrapCredentials := r.Header.Values(bootstrapHeader)
		if len(bootstrapCredentials) != 1 || !s.consumeBootstrap(bootstrapCredentials[0]) {
			writeControlError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return true
		}
		s.setLaunchCookie(w)
		writeControlJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeControlError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
	return true
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	setControlSecurityHeaders(w)
	if rejectRequestBody(w, r) {
		return
	}
	switch r.URL.Path {
	case ControlPath + "health":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeControlJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case ControlPath + "tabs/heartbeat":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		err := s.tabs.heartbeat(r.Header.Get(TabHeader), s.clock.Now())
		switch {
		case errors.Is(err, errInvalidTabID):
			writeControlError(w, http.StatusBadRequest, "invalid_tab", "A valid tab identifier is required")
		case errors.Is(err, ErrTooManyTabs):
			writeControlError(w, http.StatusTooManyRequests, "too_many_tabs", "Too many browser tabs are connected")
		case err != nil:
			writeControlError(w, http.StatusInternalServerError, "tab_tracking_failed", "Could not record browser presence")
		default:
			writeControlJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
	case ControlPath + "tabs/disconnect":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if err := s.tabs.disconnect(r.Header.Get(TabHeader), s.clock.Now()); err != nil {
			writeControlError(w, http.StatusBadRequest, "invalid_tab", "A valid tab identifier is required")
			return
		}
		writeControlJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case ControlPath + "shutdown":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		err := s.checkGuard(r.Context(), ShutdownBrowserRequest)
		var denied *ShutdownDeniedError
		switch {
		case err == nil:
			writeControlJSON(w, http.StatusAccepted, map[string]bool{"ok": true, "shutting_down": true})
			go s.beginStop(ShutdownBrowserRequest)
		case errors.As(err, &denied) && denied.valid():
			writeControlError(w, http.StatusConflict, denied.Code, denied.Message)
		default:
			writeControlError(w, http.StatusInternalServerError, "shutdown_guard_failed", "Could not determine whether shutdown is safe")
		}
	case ControlPath + "client.js":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(browserClient)
	default:
		writeControlError(w, http.StatusNotFound, "not_found", "Control endpoint not found")
	}
}

func (s *Server) checkGuard(ctx context.Context, reason ShutdownReason) error {
	if s.guard == nil {
		return nil
	}
	err := s.guard.CheckShutdown(ctx, ShutdownRequest{Reason: reason, Tabs: s.Tabs()})
	if err == nil {
		return nil
	}
	var denied *ShutdownDeniedError
	if errors.As(err, &denied) && !denied.valid() {
		return fmt.Errorf("singleserve: invalid shutdown denial: %w", err)
	}
	return err
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeControlError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	return false
}

type controlErrorResponse struct {
	OK    bool         `json:"ok"`
	Error controlError `json:"error"`
}

type controlError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeControlError(w http.ResponseWriter, status int, code, message string) {
	writeControlJSON(w, status, controlErrorResponse{OK: false, Error: controlError{Code: code, Message: message}})
}

func writeControlJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setControlSecurityHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func setControlSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func requestHasBody(r *http.Request) bool {
	return r != nil && (r.ContentLength != 0 || len(r.TransferEncoding) > 0 || (r.Body != nil && r.Body != http.NoBody))
}

func rejectRequestBody(w http.ResponseWriter, r *http.Request) bool {
	if !requestHasBody(r) {
		return false
	}
	// Control requests never need a body. Close instead of letting net/http
	// drain a deliberately slow body to preserve keep-alive.
	w.Header().Set("Connection", "close")
	_ = http.NewResponseController(w).SetReadDeadline(time.Now())
	writeControlError(w, http.StatusRequestEntityTooLarge, "request_body_not_allowed", "Request body is not allowed")
	return true
}
