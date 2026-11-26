package web

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
	"time"
)

//go:embed dist
var dashboardFiles embed.FS

func dashboardFile(w http.ResponseWriter, r *http.Request, name string) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	data, err := dashboardFiles.ReadFile("dist/" + name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Errorf("asset not found"))
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
