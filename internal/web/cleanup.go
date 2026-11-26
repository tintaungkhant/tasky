package web

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/store"
)

func (s *Server) cleanupPreview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		RootID   string
		Mappings []store.Worktree
		Options  app.RemoveOptions
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || len(body.Mappings) == 0 {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity and at least one mapping are required"))
		return
	}
	for _, mapping := range body.Mappings {
		if mapping.ID == "" || mapping.TaskID == "" || mapping.RepoID == "" || mapping.Identity == "" || !filepath.IsAbs(mapping.Path) {
			writeError(w, 400, "invalid_input", fmt.Errorf("complete mappings with absolute recorded paths are required"))
			return
		}
	}
	s.withConfiguredContext(w, body.RootID, func(c *app.Context) {
		review, err := c.ReviewCleanup(body.Mappings, body.Options)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, review)
	})
}

func (s *Server) cleanup(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Review  app.CleanupReview
		Confirm bool
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if !body.Confirm || body.Review.RootID == "" || len(body.Review.Targets) == 0 {
		writeError(w, 400, "invalid_input", fmt.Errorf("confirm a complete cleanup review"))
		return
	}
	seen := map[string]bool{}
	for _, target := range body.Review.Targets {
		m := target.Mapping
		if m.ID == "" || m.TaskID == "" || m.RepoID == "" || m.Identity == "" || target.RepoIdentity == "" || !filepath.IsAbs(m.Path) || seen[m.ID] {
			writeError(w, 400, "invalid_input", fmt.Errorf("cleanup review must contain unique complete mappings with absolute paths and verified identities"))
			return
		}
		seen[m.ID] = true
	}
	s.withConfiguredContext(w, body.Review.RootID, func(c *app.Context) {
		result, err := c.ExecuteCleanup(body.Review)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		// A validated operation can partially complete; return its structured result.
		writeJSON(w, 200, result)
	})
}

func (s *Server) pending(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q) != 1 || len(q["root"]) != 1 || q.Get("root") == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("supply exactly one Root identity"))
		return
	}
	s.withAssociationContext(w, q.Get("root"), "", func(c *app.Context) {
		creations, err := c.Store.ListCreations(c.Root.ID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		cleanups, err := c.Store.ListCleanups(c.Root.ID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		if creations == nil {
			creations = []store.Creation{}
		}
		if cleanups == nil {
			cleanups = []store.Cleanup{}
		}
		writeJSON(w, 200, struct {
			Creations []store.Creation
			Cleanups  []store.Cleanup
		}{creations, cleanups})
	})
}

func (s *Server) recover(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		RootID, ID string
		Options    app.RecoveryOptions
		Confirm    bool
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if !body.Confirm || body.RootID == "" || body.ID == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity, pending operation ID, and confirmation are required"))
		return
	}
	if body.Options.Forget && (body.Options.ForceWorktree || body.Options.ForceBranch) {
		writeError(w, 400, "invalid_input", fmt.Errorf("forget cannot be combined with force options"))
		return
	}
	s.withAssociationContext(w, body.RootID, "", func(c *app.Context) {
		creations, err := c.Store.ListCreations(c.Root.ID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		for _, op := range creations {
			if op.ID == body.ID && (body.Options.ForceWorktree || body.Options.ForceBranch) {
				writeError(w, 400, "invalid_input", fmt.Errorf("force options apply only to cleanup operations"))
				return
			}
		}
		completed, err := c.Recover(body.ID, body.Options)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		message := "Pending creation recovered."
		if body.Options.Forget {
			message = "Pending operation forgotten; Git data was not changed."
		} else if completed {
			message = "Pending cleanup recovered."
		}
		writeJSON(w, 200, struct {
			Message string `json:"Message"`
		}{message})
	})
}
