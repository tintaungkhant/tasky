Applies to: branches, secrets, staging, committing, commit messages, and worktrees. Commit and push permission is in `autonomy.md`.

# Git

- Before starting work, check which branch you are on. Whether to work on the current branch or create a new one depends on the project and the task: follow what `AGENTS.md` and the user say, and ask the user when unsure.
- Never put secrets (API keys, tokens, passwords, private keys) in code, docs or commits. Read them from environment variables. If you find a secret in a file you are about to commit, stop and tell the user.
- Stage only the files you changed, with explicit paths (`git add <paths>`, not `git add -A`). Other changes in the tree may be the user's work.
- Each commit is one coherent step that passes the checks in `AGENTS.md`. Keep a change, its tests and its docs in the same commit.
- Add no AI attribution: no `Co-Authored-By` line and no 'Generated with' line.

## Commit messages

Use Conventional Commits: `<type>(<scope>): <summary>`. Types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `style`, `build`, `ci`, `chore`, `revert`. Scope is the area touched (`auth`, `cart`, `api`).

Keep the title short and to the point: what changed, in the imperative, under 72 characters, no trailing period. Example: `feat(auth): add password reset`.

When the commit works on a ticket, end the title with the ticket ID: `(#12)` for GitHub issues, `(<feature-slug>/NN)` for local tickets, the tool's own ID otherwise. Example: `feat(cart): add coupon (#12)`.

Write the title only, no body.

## Worktrees

Create worktrees only inside the project, under `.worktree/` (`git worktree add .worktree/<branch> -b <branch>`). Never create them in `/tmp` or another outside folder: they get forgotten there and fill the disk. `.worktree/` is in `.gitignore`. When a worktree's branch is merged or dropped, remove it with `git worktree remove .worktree/<branch>`.
