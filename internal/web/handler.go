package web

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, struct {
		Error apiError `json:"error"`
	}{apiError{Code: code, Message: err.Error()}})
}

// Preserve operation messages, including partial completion and recovery IDs.
func writeOperationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err)
	default:
		writeError(w, http.StatusInternalServerError, "operation_failed", err)
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method || (method == http.MethodGet && r.Method == http.MethodHead) {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", fmt.Errorf("use %s for this endpoint", method))
	return false
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		dashboardFile(w, r, "index.html")
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		dashboardFile(w, r, strings.TrimPrefix(r.URL.Path, "/"))
	})
	mux.HandleFunc("/api/session", s.session)
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{"ok"})
	})
	mux.HandleFunc("/api/context", s.context)
	mux.HandleFunc("/api/roots", s.roots)
	mux.HandleFunc("/api/folders", s.folders)
	mux.HandleFunc("/api/preferences", s.preferences)
	mux.HandleFunc("/api/import", s.importLegacy)
	mux.HandleFunc("/api/registration/roots", s.registerRoot)
	mux.HandleFunc("/api/registration/repos", s.registerRepo)
	mux.HandleFunc("/api/repos", s.repos)
	mux.HandleFunc("/api/tasks", s.tasks)
	mux.HandleFunc("/api/tasks/options", s.creationOptions)
	mux.HandleFunc("/api/tasks/create", s.createWorktree)
	mux.HandleFunc("/api/tasks/rename", s.renameTask)
	mux.HandleFunc("/api/tasks/attach/preview", s.attachmentPreview)
	mux.HandleFunc("/api/tasks/attach", s.attachWorktree)
	mux.HandleFunc("/api/tasks/detach", s.detachWorktree)
	mux.HandleFunc("/api/tasks/reconnect", s.reconnectWorktree)
	mux.HandleFunc("/api/tasks/workspace", s.generateWorkspace)
	mux.HandleFunc("/api/tasks/cleanup/preview", s.cleanupPreview)
	mux.HandleFunc("/api/tasks/cleanup", s.cleanup)
	mux.HandleFunc("/api/pending", s.pending)
	mux.HandleFunc("/api/recover", s.recover)
	mux.HandleFunc("/api/backup", s.backup)
	mux.HandleFunc("/api/restore", s.restore)
	mux.HandleFunc("/api/registration/removal", s.removalPreview)
	mux.HandleFunc("/api/registration/remove", s.removeRoot)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Errorf("endpoint not found"))
	})
	csp := "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	host := strings.TrimPrefix(s.origin, "http://")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", csp)
		if r.Host != host {
			writeError(w, http.StatusForbidden, "invalid_host", fmt.Errorf("use the Dashboard URL printed by tasky serve"))
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != s.origin {
			writeError(w, http.StatusForbidden, "invalid_origin", fmt.Errorf("cross-origin requests are not allowed"))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && origin != s.origin {
			writeError(w, http.StatusForbidden, "invalid_origin", fmt.Errorf("state-changing requests require the dashboard origin"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			site := r.Header.Get("Sec-Fetch-Site")
			if site != "" && site != "same-origin" && site != "none" {
				writeError(w, http.StatusForbidden, "invalid_origin", fmt.Errorf("cross-origin requests are not allowed"))
				return
			}
			if r.URL.Path != "/api/session" {
				cookie, err := r.Cookie(s.cookieName)
				if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.sessionToken)) != 1 {
					writeError(w, http.StatusUnauthorized, "unauthorized", fmt.Errorf("open the Dashboard URL printed by tasky serve to connect"))
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) roots(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("Roots listing does not accept query parameters"))
		return
	}
	source, err := store.Open(app.DatabasePath(s.configPath))
	if err != nil {
		writeOperationError(w, err)
		return
	}
	defer source.Close()
	roots, err := source.ListRoots()
	if err != nil {
		writeOperationError(w, err)
		return
	}
	if roots == nil {
		roots = []store.Root{}
	}
	writeJSON(w, http.StatusOK, roots)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "invalid_content_type", fmt.Errorf("session requests require application/json"))
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("invalid session request"))
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("session request must contain one JSON value"))
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Token), []byte(s.accessToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", fmt.Errorf("invalid access link; use the Dashboard URL printed by tasky serve"))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName, Value: s.sessionToken, Path: "/api/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) context(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("invalid context query"))
		return
	}
	for key, values := range query {
		if (key != "root" && key != "repo") || len(values) != 1 {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("context accepts one root and one repo selector"))
			return
		}
	}
	source, err := store.Open(app.DatabasePath(s.configPath))
	if err != nil {
		writeOperationError(w, err)
		return
	}
	defer source.Close()
	selected, err := (registry.Registry{Store: source}).Resolve(s.cwd, query.Get("root"), query.Get("repo"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_context", err)
		return
	}
	writeJSON(w, http.StatusOK, selected)
}
