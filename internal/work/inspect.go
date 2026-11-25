package work

import (
	"fmt"
	"os"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

type State struct{ Status, Branch, Changes, Detail string }

func (m Manager) Inspect(repo store.Repo, wt store.Worktree) State {
	if _, err := os.Stat(wt.Path); err != nil {
		status := "inaccessible"
		if os.IsNotExist(err) {
			status = "missing"
		}
		return State{Status: status, Detail: fmt.Sprintf("%v; locate the checkout with git worktree list, repair its registration if necessary, then tasky reconnect", err)}
	}
	if err := m.Verify(repo, wt.Path); err != nil {
		return State{Status: "invalid", Detail: fmt.Sprintf("%v; inspect git worktree list and git worktree repair before reconnecting", err)}
	}
	identity, err := git.WorktreeIdentity(wt.Path)
	if err != nil {
		return State{Status: "invalid", Detail: err.Error()}
	}
	if wt.Identity == "" {
		return State{Status: "unverified", Detail: "mapping predates identity verification; review the checkout, then reconnect with --accept-identity"}
	}
	if identity != wt.Identity {
		return State{Status: "invalid", Detail: "identity mismatch; this is a different worktree; reconnect the original checkout or detach this stale mapping"}
	}
	branch, err := git.CurrentBranch(wt.Path)
	if err != nil {
		return State{Status: "inaccessible", Detail: err.Error()}
	}
	if branch == "" {
		branch = "(detached HEAD)"
	}
	changes, err := git.LocalChanges(wt.Path)
	if err != nil {
		return State{Status: "inaccessible", Detail: err.Error()}
	}
	return State{Status: "ok", Branch: branch, Changes: changes}
}

func (m Manager) Reconnect(repo store.Repo, wt store.Worktree, path string, acceptIdentity bool) error {
	canonical, _, err := git.VerifyWorktree(path)
	if err != nil {
		return err
	}
	if err := m.Verify(repo, canonical); err != nil {
		return err
	}
	identity, err := git.WorktreeIdentity(canonical)
	if err != nil {
		return err
	}
	if wt.Identity == "" && !acceptIdentity {
		return fmt.Errorf("original identity was not recorded; review this checkout and use --accept-identity to enroll it")
	}
	return m.Store.Reconnect(wt, canonical, identity)
}
