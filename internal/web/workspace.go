package web

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/store"
)

// Operations writing managed output need the same effective settings as the CLI.
func (s *Server) withConfiguredContext(w http.ResponseWriter, rootID string, run func(*app.Context)) {
	s.withStore(w, func(db *store.Store) {
		if _, err := rootByID(db, rootID); err != nil {
			registrationError(w, err)
			return
		}
		c, err := app.OpenRegistered(s.configPath, s.cwd, rootID)
		if err != nil {
			registrationError(w, err)
			return
		}
		defer c.Store.Close()
		if !filepath.IsAbs(c.Config.TasksDir) {
			writeError(w, 400, "invalid_input", fmt.Errorf("Tasks Directory must be absolute; review YAML preferences in Registration"))
			return
		}
		run(c)
	})
}

func (s *Server) generateWorkspace(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{ RootID, TaskID string }
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.RootID == "" || body.TaskID == "" {
		writeError(w, 400, "invalid_input", fmt.Errorf("Root identity and internal Task identity are required"))
		return
	}
	s.withConfiguredContext(w, body.RootID, func(c *app.Context) {
		result, err := c.GenerateWorkspaceForTask(body.TaskID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}
