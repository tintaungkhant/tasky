package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type folderEntry struct{ Name, Path string }

func (s *Server) folders(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q) > 1 || (len(q) == 1 && len(q["path"]) != 1) {
		writeError(w, 400, "invalid_input", fmt.Errorf("folder browsing accepts one optional absolute path"))
		return
	}
	path := q.Get("path")
	if path == "" {
		path = s.cwd
	}
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		writeError(w, 400, "invalid_input", fmt.Errorf("folder path must be absolute"))
		return
	}
	path, err = filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		folderError(w, err)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		folderError(w, err)
		return
	}
	if !info.IsDir() {
		writeError(w, 400, "invalid_input", fmt.Errorf("%s is not a directory", path))
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		folderError(w, err)
		return
	}
	directories := []folderEntry{}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(child)
			isDir = err == nil && info.IsDir()
		}
		if isDir {
			directories = append(directories, folderEntry{entry.Name(), child})
		}
	}
	roots, err := folderRoots()
	if err != nil {
		folderError(w, err)
		return
	}
	locations := []folderEntry{{"Starting folder", s.cwd}}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) && home != s.cwd {
		locations = append(locations, folderEntry{"Home", home})
	}
	for _, root := range roots {
		locations = append(locations, folderEntry{root, root})
	}
	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}
	writeJSON(w, 200, struct {
		Path, Parent           string
		Directories, Locations []folderEntry
	}{path, parent, directories, locations})
}

func folderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeError(w, 404, "not_found", err)
	case errors.Is(err, os.ErrPermission):
		writeError(w, 403, "permission_denied", err)
	default:
		writeOperationError(w, err)
	}
}
