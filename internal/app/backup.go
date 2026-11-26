package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
)

type RestoreWarning struct{ Kind, ID, Path, Status, Detail string }
type RestoreReport struct {
	Warnings         []RestoreWarning
	WorktreeWarnings int
}

func Restore(s *store.Store, b store.MappingBackup) (RestoreReport, error) {
	var report RestoreReport
	if err := s.Restore(b); err != nil {
		return report, err
	}
	for _, root := range b.Roots {
		if info, err := os.Stat(root.Path); err != nil || !info.IsDir() {
			report.Warnings = append(report.Warnings, RestoreWarning{Kind: "Root", ID: root.ID, Path: root.Path})
		}
	}
	repos := map[string]store.Repo{}
	for _, repo := range b.Repos {
		repos[repo.ID] = repo
		if _, _, err := git.VerifyWorktree(repo.Path); err != nil {
			report.Warnings = append(report.Warnings, RestoreWarning{Kind: "Repo", ID: repo.ID, Path: repo.Path, Detail: err.Error()})
		}
	}
	for _, wt := range b.Worktrees {
		state := (work.Manager{Store: s}).Inspect(repos[wt.RepoID], wt)
		if state.Status != "ok" {
			report.WorktreeWarnings++
			report.Warnings = append(report.Warnings, RestoreWarning{Kind: "worktree", ID: wt.ID, Path: wt.Path, Status: state.Status, Detail: state.Detail})
		}
	}
	return report, nil
}

func WriteMappingBackup(path string, b store.MappingBackup) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return PublishNewFile(path, append(data, '\n'))
}

func ReadMappingBackup(path string) (store.MappingBackup, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.MappingBackup{}, fmt.Errorf("open mapping backup: %w", err)
	}
	defer f.Close()
	// Bound untrusted input before decoding; mapping backups contain metadata only.
	const maxBackupSize = 64 << 20
	info, err := f.Stat()
	if err != nil {
		return store.MappingBackup{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxBackupSize {
		return store.MappingBackup{}, fmt.Errorf("mapping backup must be a regular JSON file of at most 64 MiB")
	}
	return DecodeMappingBackup(f)
}

// DecodeMappingBackup validates the same bounded JSON input for files and uploads.
func DecodeMappingBackup(input io.Reader) (store.MappingBackup, error) {
	const maxBackupSize = 64 << 20
	data, err := io.ReadAll(io.LimitReader(input, maxBackupSize+1))
	if err != nil {
		return store.MappingBackup{}, fmt.Errorf("read mapping backup: %w", err)
	}
	if len(data) > maxBackupSize {
		return store.MappingBackup{}, fmt.Errorf("mapping backup must be a regular JSON file of at most 64 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var b store.MappingBackup
	if err := decoder.Decode(&b); err != nil {
		return b, fmt.Errorf("invalid mapping backup: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return b, fmt.Errorf("invalid mapping backup: trailing data")
	}
	if err := b.Validate(); err != nil {
		return b, err
	}
	return b, nil
}
