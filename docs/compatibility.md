# CLI compatibility and verification

`add`, `list`, `delete`, `vscode`, and shell completion remain available.
Registered Roots use persisted identity; unadopted YAML Roots retain directory
discovery. [Import](adoption.md) adopts existing files without relocating them.
Registered commands use current Repo membership or unambiguous Root containment;
`--root` and `add --repo` provide explicit selection. Duplicate labels require IDs.

Necessary compatibility changes are opaque paths for new worktrees, arbitrary
Task names stored as metadata, live identity checks for registered commands, and
preservation of differing workspace files in both modes. Old binaries cannot read
the newer schema or managed layout. [Task workflows](tasks.md) describe creation
and interrupted creation; [cleanup](cleanup.md) describes separate effects,
overrides, partial results, and recovery. [Backup/restore](backup.md) recovers
mappings; keep YAML separately. Mapping restore never restores source files.

## Git independence

Tasky records associations and operation intent; Git owns branches, checkout
registrations, and working changes. No Tasky daemon or Git hooks are required.
Stopping Tasky or losing its database does not change Git data. Continue with
`git status`, `git switch`, commits, and `git worktree list/move/remove` directly.
Review dirty work before removal. Tasky itself does not provide commit, push,
pull, merge, rebase, PR, or CI integration.

`detach` and `root remove` forget mappings only. Before forgetting all mappings,
make a mapping backup if you may return. A later import can adopt legacy paths;
managed mappings require a backup or explicit attachment. Direct Git changes
can make stored paths stale; inspect and reconnect before further Tasky cleanup.

## Automated behavior coverage

Run `go build ./...`, `go vet ./...`, and `go test ./...`.
Tests use disposable real Git Repos and databases. Interactive `huh` paths and
editor rendering are not automated. The CLI emits standard workspace JSON;
tests validate its actual resolved folder paths and preservation of user content.

| Contract | Behavior tests |
| --- | --- |
| Same Task ID in separate Roots; names and identity independent of paths | `cmd/editor_test.go`: `TestEditorLifecycleAndGitIndependence`; `cmd/stored_test.go`: `TestStoredCreateAndRename`; `internal/store/store_test.go`: `TestPersistenceAndRootScopedIdentity` |
| Nested/external Repos and existing worktrees, no relocation | `cmd/registry_test.go`: `TestRegistryContext`; `cmd/attach_test.go`: `TestAttachVerifiesRepoAndPreservesDirtyFiles` |
| Multiple Repos/worktrees and distinct `foo/bar`, `foo-bar` paths | `cmd/stored_test.go`: `TestStoredCreateAndRename`; editor lifecycle test above |
| External branches, moves, missing/invalid/replacement paths, reconnect | `cmd/inspect_test.go`: `TestInspectAndReconnect`, `TestInspectLegacyEnrollment` |
| Detach and Root removal preserve code and Git | `cmd/attach_test.go`: `TestAttachDetachAndExplicitReassignment`; `cmd/registry_test.go`: `TestRootRemovalForgetsOnlySelectedMappings` |
| Dirty/unmerged protection, workspace preservation, partial cleanup and recovery | `cmd/cleanup_test.go`: `TestCleanupDirtyProtectionAndInterruptedRecovery`, `TestCleanupRecoversBranchFailure`, `TestCleanupRecoveryRejectsReplacementRepo`; editor lifecycle test above |
| Concurrent CLI and dashboard use, interrupted creation | `cmd/editor_test.go`: `TestCLIAndDashboardConcurrentCreationAndRecovery` races CLI processes and a signed-in dashboard HTTP request against one Repo and database, then recovers persisted intent with another process; `cmd/stored_test.go`: `TestStoredCreationConcurrencyAndRecovery` |
| Concurrent publication never replaces or truncates output | `cmd/editor_test.go`: `TestConcurrentWorkspacePublication` |
| Repeat import, unchanged files/branches, ambiguity | `cmd/import_test.go`: `TestImportLegacyIsRepeatableAndPreservesWorkflow`, `TestImportAmbiguousTaskRequiresReview` |
| Backup/restore associations, unavailable DB, usable Git | `cmd/backup_test.go`: `TestBackupRestoreChecksLiveMappingsAndPreservesGit`, `TestInvalidBackupAndUnavailableDatabase`; editor lifecycle test continues through Git switch, commit, move, and removal with a blocked database |
| Dashboard backup and restore | `internal/web/backup_test.go` checks CLI snapshot compatibility, stale locations, rollback, pending blockers and the request boundary. `frontend/src/Backup.test.tsx` checks file selection, keyboard submission, errors, retries and warnings. |
| Existing command and completion contracts | `cmd/add_test.go`, `list_test.go`, `delete_test.go`, `vscode_test.go`, `completion_test.go`, and registered editor lifecycle test |
| Workspace uses verified associations, excludes arbitrary directories, preserves edits | `cmd/editor_test.go`: `TestStoredWorkspaceVerifiesEveryAssociation`, `TestLegacyWorkspacePreservesUserEditsAndIgnoresDirectories`, editor lifecycle test |
