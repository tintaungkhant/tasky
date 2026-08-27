# 01 — Project scaffold + `tasky setup`

**What to build:** A working `tasky` binary with a `setup` command. Running `tasky setup` walks the developer through adding, listing, editing, and removing Roots (name, path, tasks_dir, branch_template), persisting them to `~/.config/tasky/config.yaml`. This is the foundation every other ticket builds on: Go module, cobra command wiring, and `internal/config` (load/save YAML, root CRUD, root-match resolution by longest path prefix).

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `go.mod` initialized, `spf13/cobra` and `charmbracelet/huh` added as deps
- [ ] `tasky` binary builds and `tasky --help` shows `setup` and `add` (stub) subcommands
- [ ] `internal/config`: YAML load/save round-trip against `~/.config/tasky/config.yaml` (or `$XDG_CONFIG_HOME`)
- [ ] `internal/config`: root-matching resolves a given path to the correct configured Root by longest matching `path` prefix; returns a clear "no configured root" error when nothing matches
- [ ] `tasky setup` (huh forms): add a new Root (name, path, tasks_dir — defaults to `<path>/tasks`, branch_template — defaults to `{branch}`); list existing Roots; edit a Root; remove a Root
- [ ] Table-driven unit tests for `internal/config` covering load/save, root-matching (including nested-root longest-prefix case and no-match case)
