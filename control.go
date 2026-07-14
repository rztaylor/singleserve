package singleserve

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	// TokenHeader carries the per-launch authentication token.
	TokenHeader = "X-Singleserve-Token"
	// TabHeader identifies one browser tab instance.
	TabHeader = "X-Singleserve-Tab"
	// ControlPath is reserved for Singleserve lifecycle endpoints.
	ControlPath = "/_singleserve/"
)

//go:embed client/singleserve.js
var browserClient []byte

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
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
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
