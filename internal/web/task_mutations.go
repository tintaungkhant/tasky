package web

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

func (s *Server) withCreationContext(w http.ResponseWriter, rootID, repoID string, run func(*app.Context, store.Repo)) {
	s.withStore(w, func(db *store.Store) {
		if _, err := rootByID(db, rootID); err != nil {
			registrationError(w, err)
			return
		}
		repos, err := db.ListRepos(rootID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		for _, repo := range repos {
			if repo.ID != repoID {
				continue
			}
			s.withConfiguredContext(w, rootID, func(c *app.Context) { run(c, repo) })
			return
		}
		writeOperationError(w, store.ErrNotFound)
	})
}

func (s *Server) creationOptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q) != 2 || len(q["root"]) != 1 || len(q["repo"]) != 1 || q.Get("root") == "" || q.Get("repo") == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("supply exactly one Root identity and Repo identity"))
		return
	}
	s.withCreationContext(w, q.Get("root"), q.Get("repo"), func(c *app.Context, repo store.Repo) {
		branches, err := git.ListLocalBranches(repo.Path)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		current, err := git.CurrentBranch(repo.Path)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		if branches == nil {
			branches = []string{}
		}
		writeJSON(w, 200, struct {
			Branches                                []string
			CurrentBranch, TasksDir, BranchTemplate string
		}{branches, current, c.Config.TasksDir, c.Config.BranchTemplate})
	})
}

func (s *Server) createWorktree(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		RootID, RepoID, TaskID, Name, Branch, Base string
		AsIs                                       bool
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || body.RepoID == "" || body.TaskID == "" || body.Branch == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity, Repo identity, Task ID, and branch are required"))
		return
	}
	s.withCreationContext(w, body.RootID, body.RepoID, func(c *app.Context, repo store.Repo) {
		result, err := c.Create(app.CreateRequest{TaskID: body.TaskID, Name: body.Name, Repo: repo.ID, Branch: body.Branch, Base: body.Base, AsIs: body.AsIs})
		if err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})
}

func (s *Server) renameTask(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		RootID, ID string
		Name       *string
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || body.ID == "" || body.Name == nil {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity, internal Task identity, and name (which may be empty) are required"))
		return
	}
	s.withStore(w, func(db *store.Store) {
		if _, err := rootByID(db, body.RootID); err != nil {
			registrationError(w, err)
			return
		}
		tasks, err := db.ListTasks(body.RootID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		for _, task := range tasks {
			if task.ID != body.ID {
				continue
			}
			// Rename the scoped internal identity, never a re-used public Task ID.
			if err := db.RenameTask(task.ID, *body.Name); err != nil {
				writeOperationError(w, err)
				return
			}
			writeJSON(w, 200, struct{ Message string }{"Task renamed; identity, branches, paths, and associations preserved."})
			return
		}
		writeOperationError(w, store.ErrNotFound)
	})
}
