package web

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
)

// Association operations need persisted registration, not creation preferences.
func (s *Server) withAssociationContext(w http.ResponseWriter, rootID, repoID string, run func(*app.Context)) {
	s.withStore(w, func(db *store.Store) {
		root, err := rootByID(db, rootID)
		if err != nil {
			registrationError(w, err)
			return
		}
		if repoID != "" {
			repos, err := db.ListRepos(root.ID)
			if err != nil {
				writeOperationError(w, err)
				return
			}
			found := false
			for _, repo := range repos {
				if repo.ID == repoID {
					found = true
					break
				}
			}
			if !found {
				writeOperationError(w, store.ErrNotFound)
				return
			}
		}
		run(&app.Context{Registry: registry.Registry{Store: db}, Root: root, Cwd: s.cwd})
	})
}

func (s *Server) attachmentPreview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{ RootID, RepoID, TaskID, Name, Path string }
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || body.RepoID == "" || body.TaskID == "" || !filepath.IsAbs(body.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity, Repo identity, Task ID, and absolute checkout path are required"))
		return
	}
	s.withAssociationContext(w, body.RootID, body.RepoID, func(c *app.Context) {
		review, err := c.ReviewAttachment(body.TaskID, body.Name, body.Path, body.RepoID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, review)
	})
}

func (s *Server) attachWorktree(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Review       app.AttachmentReview
		ReassignFrom string
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	review := body.Review
	if review.RootID == "" || review.RepoID == "" || review.TaskID == "" || review.Identity == "" || !filepath.IsAbs(review.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("a complete attachment review with absolute path and verified identity is required"))
		return
	}
	s.withAssociationContext(w, review.RootID, review.RepoID, func(c *app.Context) {
		wt, err := c.AttachReviewed(review, body.ReassignFrom)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, wt)
	})
}

type mappingRequest struct {
	RootID         string
	Mapping        store.Worktree
	Path           string
	AcceptIdentity bool
}

func validMapping(w http.ResponseWriter, body mappingRequest) bool {
	wt := body.Mapping
	if body.RootID == "" || wt.ID == "" || wt.TaskID == "" || wt.RepoID == "" || !filepath.IsAbs(wt.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity and complete mapping snapshot with absolute recorded path are required"))
		return false
	}
	return true
}

func (s *Server) detachWorktree(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	// Detach does not accept reconnect-only options.
	var body struct {
		RootID  string
		Mapping store.Worktree
		Confirm bool
	}
	if !decodeRequest(w, r, &body) || !validMapping(w, mappingRequest{RootID: body.RootID, Mapping: body.Mapping}) {
		return
	}
	if !body.Confirm {
		writeError(w, 400, "invalid_input", fmt.Errorf("confirm detachment of the reviewed association; files and branches will be preserved"))
		return
	}
	s.withAssociationContext(w, body.RootID, body.Mapping.RepoID, func(c *app.Context) {
		if err := c.Store.DetachReviewed(c.Root.ID, body.Mapping); err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, struct{ Message string }{"Association detached; files and Git branches preserved."})
	})
}

func (s *Server) reconnectWorktree(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body mappingRequest
	if !decodeRequest(w, r, &body) || !validMapping(w, body) {
		return
	}
	if !filepath.IsAbs(body.Path) {
		writeError(w, 400, "invalid_input", fmt.Errorf("an absolute checkout path is required"))
		return
	}
	s.withAssociationContext(w, body.RootID, body.Mapping.RepoID, func(c *app.Context) {
		if err := c.ReconnectReviewed(body.Mapping, body.Path, body.AcceptIdentity); err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, struct{ Message string }{"Association reconnected; files and Git branches preserved."})
	})
}
