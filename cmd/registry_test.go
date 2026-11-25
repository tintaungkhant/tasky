package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
)

func registryRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRegistryCommands()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRegistryContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TASKY_CONFIG", configPath)
	legacy := []byte("roots: []\n# preserve user configuration\n")
	if err := os.WriteFile(configPath, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	root := filepath.Join(base, "root")
	repo := filepath.Join(root, "nested", "repo")
	external := filepath.Join(base, "external")
	initTestRepo(t, repo)
	initTestRepo(t, external)
	before := gitSnapshot(t, external)
	for _, args := range [][]string{
		{"root", "add", "company", root},
		{"repo", "add", "nested", repo, "--root", "company"},
		{"repo", "add", "outside", external, "--root", "company"},
	} {
		if _, err := registryRun(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	if after := gitSnapshot(t, external); after != before {
		t.Fatal("registration changed the original Repo state")
	}
	chdir(t, root)
	if _, err := registryRun(t, "context"); err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("expected actionable ambiguity: %v", err)
	}
	out, err := registryRun(t, "context", "--root", "company", "--repo", "outside")
	if err != nil || !strings.Contains(out, external) {
		t.Fatalf("explicit context: %s %v", out, err)
	}
	wt := filepath.Join(base, "linked")
	runGit(t, external, "worktree", "add", "-b", "feature", wt)
	withWorktree := gitSnapshot(t, external)
	if _, err := registryRun(t, "repo", "add", "duplicate", wt, "--root", "company"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("linked checkout must identify the same Repo: %v", err)
	}
	chdir(t, wt)
	out, err = registryRun(t, "context")
	if err != nil || !strings.Contains(out, "outside") {
		t.Fatalf("worktree context: %s %v", out, err)
	}
	chdir(t, base)
	if _, err := registryRun(t, "root", "remove", "company"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{repo, external, wt} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, wt, "diff", "--exit-code")
	if after := gitSnapshot(t, external); after != withWorktree {
		t.Fatalf("removal changed Git state: %q != %q", after, withWorktree)
	}
	if out, err := registryRun(t, "root", "list"); err != nil || strings.Contains(out, "company") {
		t.Fatalf("Root removal: %s %v", out, err)
	}
	if data, err := os.ReadFile(configPath); err != nil || !bytes.Equal(data, legacy) {
		t.Fatalf("registration changed YAML: %q %v", data, err)
	}
}

func gitSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var snapshot string
	for _, args := range [][]string{{"status", "--porcelain"}, {"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"}, {"worktree", "list", "--porcelain"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git snapshot: %s %v", out, err)
		}
		snapshot += string(out)
	}
	return snapshot
}

func openRegistry(t *testing.T, path string) registry.Registry {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return registry.Registry{Store: s}
}

func TestRegistrySelection(t *testing.T) {
	base := t.TempDir()
	r := openRegistry(t, filepath.Join(base, "db.sqlite3"))
	outer, err := r.AddRoot("same", base)
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(base, "nested")
	initTestRepo(t, nested)
	inner, err := r.AddRoot("same", nested)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ResolveRoot(nested, "same"); err == nil {
		t.Fatal("duplicate Root names must be ambiguous")
	}
	if _, err := r.ResolveRoot(nested, ""); err == nil {
		t.Fatal("overlapping Root directories must be ambiguous")
	}
	first, err := r.AddRepo(outer, "same", nested)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := r.Resolve(nested, "", "")
	if err != nil || ctx.Root.ID != outer.ID || ctx.Repo.ID != first.ID {
		t.Fatalf("membership must precede containment: %+v %v", ctx, err)
	}
	// A nested Git Repo resolves independently of its containing Repo.
	child := filepath.Join(nested, "child")
	initTestRepo(t, child)
	second, err := r.AddRepo(outer, "same", child)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = r.Resolve(child, "", "")
	if err != nil || ctx.Repo.ID != second.ID {
		t.Fatalf("nested Repo: %+v %v", ctx, err)
	}
	if _, err := r.Resolve(child, outer.ID, "same"); err == nil {
		t.Fatal("duplicate Repo names must be ambiguous")
	}
	ctx, err = r.Resolve(base, "", first.ID)
	if err != nil || ctx.Repo.ID != first.ID {
		t.Fatalf("Repo ID from outside Repo: %+v %v", ctx, err)
	}
	if _, err := r.AddRepo(inner, "shared", nested); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(nested, "", ""); err == nil {
		t.Fatal("shared Repo requires explicit Root")
	}
	ctx, err = r.Resolve(nested, inner.ID, "")
	if err != nil || ctx.Root.ID != inner.ID {
		t.Fatalf("explicit Root: %+v %v", ctx, err)
	}
	if _, err := r.Resolve(base, outer.ID, "missing"); err == nil {
		t.Fatal("unknown Repo must fail")
	}
}

func TestRegistryConcurrentAndFailedRegistration(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "db.sqlite3")
	a, b := openRegistry(t, path), openRegistry(t, path)
	root, err := a.AddRoot("company", base)
	if err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(base, "repo")
	initTestRepo(t, repoPath)
	before := gitSnapshot(t, repoPath)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, r := range []registry.Registry{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.AddRepo(root, "repo", repoPath); results <- err }()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, store.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
	if _, err := a.AddRepo(root, "invalid", base); err == nil {
		t.Fatal("non-Git Repo accepted")
	}
	if _, err := a.AddRepo(root, "", repoPath); err == nil {
		t.Fatal("empty Repo name accepted")
	}
	if _, err := a.AddRoot("", base); err == nil {
		t.Fatal("empty Root name accepted")
	}
	if _, err := a.AddRoot("invalid", ""); err == nil {
		t.Fatal("empty Root path accepted")
	}
	if _, err := a.AddRepo(root, "invalid", ""); err == nil {
		t.Fatal("empty Repo path accepted")
	}
	if _, err := a.AddRoot("duplicate", filepath.Join(base, ".")); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate Root: %v", err)
	}
	repos, err := a.Store.ListRepos(root.ID)
	if err != nil || len(repos) != 1 {
		t.Fatalf("partial registration: %+v %v", repos, err)
	}
	if after := gitSnapshot(t, repoPath); after != before {
		t.Fatal("registration changed Git state")
	}
	if err := a.Store.RemoveRoot(root.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AddRepo(root, "stale", repoPath); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale Root must fail: %v", err)
	}
	repos, err = a.Store.ListRepos(root.ID)
	if err != nil || len(repos) != 0 {
		t.Fatalf("orphan registration: %+v %v", repos, err)
	}
}

func TestRootRemovalForgetsOnlySelectedMappings(t *testing.T) {
	base := t.TempDir()
	r := openRegistry(t, filepath.Join(base, "db.sqlite3"))
	root, err := r.AddRoot("one", base)
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.AddRoot("two", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(base, "repo")
	initTestRepo(t, repoPath)
	repo, err := r.AddRepo(root, "repo", repoPath)
	if err != nil {
		t.Fatal(err)
	}
	task, err := r.Store.CreateTask(root.ID, "T-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Store.Associate(task.ID, repo.ID, repoPath); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Store.CreateTask(other.ID, "T-1", "keep"); err != nil {
		t.Fatal(err)
	}
	before := gitSnapshot(t, repoPath)
	if err := r.Store.RemoveRoot(root.ID); err != nil {
		t.Fatal(err)
	}
	roots, err := r.Store.ListRoots()
	if err != nil || len(roots) != 1 || roots[0].ID != other.ID {
		t.Fatalf("Roots: %+v %v", roots, err)
	}
	tasks, err := r.Store.ListTasks(other.ID)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("unrelated Tasks: %+v %v", tasks, err)
	}
	worktrees, err := r.Store.ListWorktrees(task.ID)
	if err != nil || len(worktrees) != 0 {
		t.Fatalf("orphan worktrees: %+v %v", worktrees, err)
	}
	if err := r.Store.RemoveRoot(root.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("repeat removal: %v", err)
	}
	if after := gitSnapshot(t, repoPath); after != before {
		t.Fatal("removal changed Git state")
	}
}
