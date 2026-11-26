package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
)

func TestApplicationOperationsDoNotDependOnCLIState(t *testing.T) {
	root, repo, _ := storedSetup(t)
	configPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	// A server can retain explicit paths and context while process state differs.
	chdir(t, t.TempDir())
	t.Setenv("TASKY_CONFIG", filepath.Join(t.TempDir(), "unrelated.yaml"))
	c, err := app.OpenRegistered(configPath, root, "company")
	if err != nil || c == nil {
		t.Fatalf("open explicit context: %v", err)
	}
	defer c.Store.Close()
	if count, err := app.Import(configPath); err != nil || count != 1 {
		t.Fatalf("explicit import: %d %v", count, err)
	}
	created, err := c.Create(app.CreateRequest{TaskID: "shared", Name: "Refund / ပြန်အမ်းခြင်း @ web", Repo: "project-a", Branch: "foo/bar"})
	if err != nil || created.Branch != "task/shared/foo/bar" {
		t.Fatalf("create: %+v %v", created, err)
	}
	runGit(t, repo, "branch", "existing")
	second, err := c.Create(app.CreateRequest{TaskID: "shared", Name: "ignored", Repo: "project-a", Branch: "existing"})
	if err != nil || second.Branch != "existing" {
		t.Fatalf("reuse: %+v %v", second, err)
	}
	task, err := c.Task("shared")
	if err != nil || task.Name != "Refund / ပြန်အမ်းခြင်း @ web" {
		t.Fatalf("name: %+v %v", task, err)
	}
	workspace, err := c.GenerateWorkspace("shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(readWorkspace(t, workspace.Path).Folders) != 2 {
		t.Fatal("workspace omitted associations")
	}
	if err := c.Rename("shared", "New / name"); err != nil {
		t.Fatal(err)
	}
	// The ordinary CLI sees exactly the same state through the shared operations.
	t.Setenv("TASKY_CONFIG", configPath)
	out, err := storedExec(t, "list", "shared", "--root", "company")
	if err != nil || !strings.Contains(out, created.Path) || !strings.Contains(out, "New / name") {
		t.Fatalf("CLI parity: %s %v", out, err)
	}
	out, err = storedExec(t, "vscode", "shared", "--root", "company")
	if err != nil || !strings.Contains(out, "Workspace file is current:") {
		t.Fatalf("workspace parity: %s %v", out, err)
	}
	// Relative paths use the caller's context rather than process-wide chdir.
	external := filepath.Join(root, "external")
	runGit(t, repo, "worktree", "add", "-b", "external", external)
	wt, err := c.Attach("external", "external", "", "", "")
	if err != nil || wt.Path != external {
		t.Fatalf("relative attach: %+v %v", wt, err)
	}
	moved := filepath.Join(root, "moved")
	runGit(t, repo, "worktree", "move", external, moved)
	if _, err := c.Reconnect("external", "external", "moved", false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Detach("external", "moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatal("detach changed checkout")
	}
	// Direct application calls keep the same dirty-worktree/recovery boundary.
	_, mappings, _, err := c.Worktrees("shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.Path, "keep.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(mappings[created.Path], app.RemoveOptions{}); err == nil {
		t.Fatal("application bypassed dirty protection")
	}
	pending, err := c.Store.ListCleanups(c.Root.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	if cleanup, err := c.Recover(pending[0].ID, app.RecoveryOptions{Forget: true}); err != nil || cleanup {
		t.Fatalf("forget: %t %v", cleanup, err)
	}
	if data, _ := os.ReadFile(filepath.Join(created.Path, "keep.txt")); string(data) != "keep" {
		t.Fatal("forget lost source")
	}
}
