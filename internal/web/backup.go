package web

import (
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/store"
)

func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("backup covers all Roots and does not accept query parameters"))
		return
	}
	s.withStore(w, func(db *store.Store) {
		snapshot, err := db.Backup()
		if err != nil {
			writeBackupError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="tasky-mappings.json"`)
		writeJSON(w, http.StatusOK, snapshot)
	})
}

func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Errorf("restore does not accept query parameters"))
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "invalid_content_type", fmt.Errorf("restore requires an application/json mapping snapshot"))
		return
	}
	snapshot, err := app.DecodeMappingBackup(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err)
		return
	}
	s.withStore(w, func(db *store.Store) {
		report, err := app.Restore(db, snapshot)
		if err != nil {
			writeBackupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
}

func writeBackupError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrBackupPending) {
		writeError(w, http.StatusConflict, "pending_operations", fmt.Errorf("pending creation or cleanup blocks backup and restore; select each Root in Recovery and resolve its pending operations before retrying"))
		return
	}
	writeOperationError(w, err)
}
