package web

import (
	"net/http"

	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
)

type inspectedWorktree struct {
	Mapping store.Worktree
	Repo    store.Repo
	State   work.State
}

type inspectedTask struct {
	Task      store.Task
	Worktrees []inspectedWorktree
}

func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	id, ok := rootQuery(w, r)
	if !ok {
		return
	}
	s.withStore(w, func(db *store.Store) {
		root, err := rootByID(db, id)
		if err != nil {
			registrationError(w, err)
			return
		}
		tasks, err := db.ListTasks(root.ID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		repos, err := db.ListRepos(root.ID)
		if err != nil {
			writeOperationError(w, err)
			return
		}
		reposByID := make(map[string]store.Repo, len(repos))
		for _, repo := range repos {
			reposByID[repo.ID] = repo
		}
		result := make([]inspectedTask, 0, len(tasks))
		manager := work.Manager{Store: db}
		for _, task := range tasks {
			mappings, err := db.ListWorktrees(task.ID)
			if err != nil {
				writeOperationError(w, err)
				return
			}
			item := inspectedTask{Task: task, Worktrees: make([]inspectedWorktree, 0, len(mappings))}
			for _, mapping := range mappings {
				repo, exists := reposByID[mapping.RepoID]
				if !exists {
					writeOperationError(w, store.ErrNotFound)
					return
				}
				item.Worktrees = append(item.Worktrees, inspectedWorktree{
					Mapping: mapping,
					Repo:    repo,
					State:   manager.Inspect(repo, mapping),
				})
			}
			result = append(result, item)
		}
		writeJSON(w, http.StatusOK, result)
	})
}
