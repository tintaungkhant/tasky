package app

import (
	"errors"
	"fmt"

	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
)

type AttachmentReview struct {
	RootID, RepoID, TaskID, Name, TargetID, Path, Identity string
	Source                                                 *store.Worktree
	SourceTask                                             *store.Task
	SourceRoot                                             *store.Root
}

func (c *Context) ReviewAttachment(taskID, name, location, repoID string) (AttachmentReview, error) {
	repo, path, identity, err := c.verifyAttachment(location, repoID)
	if err != nil {
		return AttachmentReview{}, err
	}
	review := AttachmentReview{RootID: c.Root.ID, RepoID: repo.ID, TaskID: taskID, Name: name, Path: path, Identity: identity}
	target, err := c.Store.FindTask(c.Root.ID, taskID)
	if err == nil {
		review.TargetID = target.ID
	} else if !errors.Is(err, store.ErrNotFound) {
		return review, err
	}
	review.Source, review.SourceTask, review.SourceRoot, err = c.Store.AssociationAt(path)
	return review, err
}

func (c *Context) AttachReviewed(review AttachmentReview, from string) (store.Worktree, error) {
	if review.RootID != c.Root.ID {
		return store.Worktree{}, store.ErrNotFound
	}
	repo, path, identity, err := c.verifyAttachment(review.Path, review.RepoID)
	if err != nil {
		return store.Worktree{}, err
	}
	if path != review.Path || identity != review.Identity {
		return store.Worktree{}, fmt.Errorf("checkout changed since review; review again: %w", store.ErrConflict)
	}
	return c.Store.AttachReviewed(c.Root.ID, review.TaskID, review.Name, repo.ID, path, from, identity, store.AssociationReview{TargetID: review.TargetID, Source: review.Source})
}

// ReconnectReviewed uses the inspected snapshot all the way to the Store CAS.
func (c *Context) ReconnectReviewed(wt store.Worktree, path string, accept bool) error {
	tasks, err := c.Store.ListTasks(c.Root.ID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.ID != wt.TaskID {
			continue
		}
		repos, err := c.Store.ListRepos(c.Root.ID)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if repo.ID == wt.RepoID {
				return (work.Manager{Store: c.Store}).Reconnect(repo, wt, c.location(path), accept)
			}
		}
	}
	return store.ErrNotFound
}
