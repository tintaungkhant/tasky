// Package registry validates registration and resolves explicit or directory
// context. Git owns repository identity; SQLite owns Root membership.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

type Registry struct{ Store *store.Store }
type Context struct {
	Root store.Root
	Repo store.Repo
}

func canonical(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("directory path must not be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func same(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func contains(parent, child string) bool {
	if runtime.GOOS == "windows" {
		parent, child = strings.ToLower(parent), strings.ToLower(child)
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (r Registry) AddRoot(name, path string) (store.Root, error) {
	if strings.TrimSpace(name) == "" {
		return store.Root{}, fmt.Errorf("Root name must not be empty")
	}
	path, err := canonical(path)
	if err != nil {
		return store.Root{}, fmt.Errorf("Root directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return store.Root{}, err
	}
	if !info.IsDir() {
		return store.Root{}, fmt.Errorf("Root path must be a directory")
	}
	return r.Store.CreateRoot(name, path)
}

func (r Registry) AddRepo(root store.Root, name, path string) (store.Repo, error) {
	if strings.TrimSpace(name) == "" {
		return store.Repo{}, fmt.Errorf("Repo name must not be empty")
	}
	path, err := canonical(path)
	if err != nil {
		return store.Repo{}, fmt.Errorf("Repo directory: %w", err)
	}
	path, err = git.PrimaryWorktree(path)
	if err != nil {
		return store.Repo{}, fmt.Errorf("register Repo: %w", err)
	}
	path, err = canonical(path)
	if err != nil {
		return store.Repo{}, fmt.Errorf("Repo directory: %w", err)
	}
	return r.Store.CreateRepo(root.ID, name, path)
}

// ResolveRoot requires a unique explicit selector or unique directory context.
// Repository membership takes precedence over physical Root containment.
func (r Registry) ResolveRoot(cwd, selector string) (store.Root, error) {
	roots, err := r.Store.ListRoots()
	if err != nil {
		return store.Root{}, err
	}
	var matches []store.Root
	if selector != "" {
		for _, root := range roots {
			if root.ID == selector || root.Name == selector {
				matches = append(matches, root)
			}
		}
	} else {
		cwd, err = canonical(cwd)
		if err != nil {
			return store.Root{}, fmt.Errorf("current directory: %w", err)
		}
		common, gitErr := git.CommonDir(cwd)
		if gitErr == nil {
			common, err = canonical(common)
			if err != nil {
				return store.Root{}, err
			}
			for _, root := range roots {
				repos, err := r.Store.ListRepos(root.ID)
				if err != nil {
					return store.Root{}, err
				}
				for _, repo := range repos {
					if repoMatches(repo, common) {
						matches = append(matches, root)
						break
					}
				}
			}
		}
		if len(matches) == 0 {
			for _, root := range roots {
				path, err := canonical(root.Path)
				if err == nil && contains(path, cwd) {
					matches = append(matches, root)
				}
			}
		}
	}
	if len(matches) != 1 {
		return store.Root{}, fmt.Errorf("Root selection matched %d Roots; use --root with a unique name or ID from `tasky root list` (register with `tasky root add <name> <path>`)", len(matches))
	}
	return matches[0], nil
}

func repoMatches(repo store.Repo, common string) bool {
	path, err := git.CommonDir(repo.Path)
	if err != nil {
		return false
	}
	path, err = canonical(path)
	return err == nil && same(path, common)
}

func (r Registry) Resolve(cwd, rootSelector, repoSelector string) (Context, error) {
	// An explicit Repo can identify its Root even outside every Root directory.
	if rootSelector == "" && repoSelector != "" {
		roots, err := r.Store.ListRoots()
		if err != nil {
			return Context{}, err
		}
		var matches []store.Root
		for _, root := range roots {
			repos, err := r.Store.ListRepos(root.ID)
			if err != nil {
				return Context{}, err
			}
			for _, repo := range repos {
				if repo.ID == repoSelector || repo.Name == repoSelector {
					matches = append(matches, root)
				}
			}
		}
		if len(matches) != 1 {
			return Context{}, fmt.Errorf("Repo selection matched %d registrations; use --root and --repo with unique IDs from `tasky root list` and `tasky repo list --root <ID>`", len(matches))
		}
		rootSelector = matches[0].ID
	}
	root, err := r.ResolveRoot(cwd, rootSelector)
	if err != nil {
		return Context{}, err
	}
	repos, err := r.Store.ListRepos(root.ID)
	if err != nil {
		return Context{}, err
	}
	var matches []store.Repo
	if repoSelector != "" {
		for _, repo := range repos {
			if repo.ID == repoSelector || repo.Name == repoSelector {
				matches = append(matches, repo)
			}
		}
	} else {
		common, err := git.CommonDir(cwd)
		if err == nil {
			common, err = canonical(common)
			if err != nil {
				return Context{}, err
			}
			for _, repo := range repos {
				if repoMatches(repo, common) {
					matches = append(matches, repo)
				}
			}
		}
	}
	if len(matches) != 1 {
		return Context{}, fmt.Errorf("Repo selection matched %d Repos; use --repo with a unique name or ID from `tasky repo list --root %s`", len(matches), root.ID)
	}
	return Context{Root: root, Repo: matches[0]}, nil
}
