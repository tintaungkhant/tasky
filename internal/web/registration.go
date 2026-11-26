package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
)

func decodeRequest(w http.ResponseWriter, r *http.Request, body any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "invalid_content_type", fmt.Errorf("request requires application/json"))
		return false
	}
	if r.URL.RawQuery != "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("mutation does not accept query parameters"))
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(body); err != nil {
		writeError(w, 400, "invalid_input", fmt.Errorf("invalid request: %w", err))
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "invalid_input", fmt.Errorf("request must contain one JSON value"))
		return false
	}
	return true
}

func registrationError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNotFound) {
		writeOperationError(w, err)
	} else {
		writeError(w, 400, "invalid_input", err)
	}
}

func (s *Server) withStore(w http.ResponseWriter, run func(*store.Store)) {
	db, err := store.Open(app.DatabasePath(s.configPath))
	if err != nil {
		writeOperationError(w, err)
		return
	}
	defer db.Close()
	run(db)
}

func rootQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["root"]) != 1 || query.Get("root") == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("supply exactly one root identity"))
		return "", false
	}
	return query.Get("root"), true
}

func rootByID(db *store.Store, id string) (store.Root, error) {
	roots, err := db.ListRoots()
	if err != nil {
		return store.Root{}, err
	}
	for _, root := range roots {
		if root.ID == id {
			return root, nil
		}
	}
	return store.Root{}, store.ErrNotFound
}

func (s *Server) preferences(w http.ResponseWriter, r *http.Request) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if r.URL.RawQuery != "" {
			writeError(w, 400, "invalid_input", fmt.Errorf("preferences do not accept query parameters"))
			return
		}
		cfg, err := config.LoadFile(s.configPath)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		if cfg.Roots == nil {
			cfg.Roots = []config.Root{}
		}
		writeJSON(w, 200, cfg)
		return
	}
	if !requireMethod(w, r, http.MethodPut) {
		return
	}
	var body struct {
		Expected *config.Config `json:"expected"`
		Config   *config.Config `json:"config"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Expected == nil || body.Config == nil {
		writeError(w, 400, "invalid_input", fmt.Errorf("expected preferences and new config are required"))
		return
	}
	if err := app.UpdatePreferences(s.configPath, *body.Expected, *body.Config); err != nil {
		registrationError(w, err)
		return
	}
	writeJSON(w, 200, body.Config)
}

func (s *Server) importLegacy(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{}
	if !decodeRequest(w, r, &body) {
		return
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	count, err := app.Import(s.configPath)
	if err != nil {
		registrationError(w, err)
		return
	}
	writeJSON(w, 200, struct{ Roots int }{count})
}

func (s *Server) registerRoot(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{ Name, Path string }
	if !decodeRequest(w, r, &body) {
		return
	}
	if !filepath.IsAbs(body.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root path must be absolute"))
		return
	}
	s.withStore(w, func(db *store.Store) {
		root, err := (registry.Registry{Store: db}).AddRoot(body.Name, body.Path)
		if err != nil {
			registrationError(w, err)
			return
		}
		writeJSON(w, 201, root)
	})
}

func (s *Server) registerRepo(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{ RootID, Name, Path string }
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || !filepath.IsAbs(body.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity and absolute Repo path are required"))
		return
	}
	s.withStore(w, func(db *store.Store) {
		root, err := rootByID(db, body.RootID)
		if err != nil {
			registrationError(w, err)
			return
		}
		repo, err := (registry.Registry{Store: db}).AddRepo(root, body.Name, body.Path)
		if err != nil {
			registrationError(w, err)
			return
		}
		writeJSON(w, 201, repo)
	})
}

func (s *Server) repos(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	id, ok := rootQuery(w, r)
	if !ok {
		return
	}
	s.withStore(w, func(db *store.Store) {
		if _, err := rootByID(db, id); err != nil {
			registrationError(w, err)
			return
		}
		repos, err := db.ListRepos(id)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		if repos == nil {
			repos = []store.Repo{}
		}
		writeJSON(w, 200, repos)
	})
}

func (s *Server) removalPreview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	id, ok := rootQuery(w, r)
	if !ok {
		return
	}
	s.withStore(w, func(db *store.Store) {
		preview, err := db.RootMappings(id)
		if err != nil {
			registrationError(w, err)
			return
		}
		writeJSON(w, 200, preview)
	})
}

func (s *Server) removeRoot(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Confirm  bool                       `json:"confirm"`
		Expected *store.RootMappingSnapshot `json:"expected"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if !body.Confirm || body.Expected == nil || body.Expected.Root.ID == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("confirm the exact Root mapping preview before removal"))
		return
	}
	s.withStore(w, func(db *store.Store) {
		if err := db.RemoveRootConfirmed(*body.Expected); err != nil {
			registrationError(w, err)
			return
		}
		writeJSON(w, 200, struct{ Message string }{"Root mappings forgotten. Files, worktrees, branches, and YAML preferences are preserved."})
	})
}
