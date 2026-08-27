# 05 — `tasky add` full interactive flow + confirmation

**What to build:** Running bare `tasky add` (no args) walks the developer through every decision that couldn't be inferred, using huh forms, then shows a resolved-configuration confirmation screen before touching Git. Wires together repo resolution (03), base-branch handling (04), and existing/new-branch logic behind an interactive UI — only asking for what cwd/args didn't already supply.

**Blocked by:** 03, 04

**Status:** done

- [x] `tasky add` prompts only for fields not already resolved from cwd/args/flags (per spec §5 field resolution order): repo select (if not detected/given), task-id input, base-branch select (local branches, current branch pre-selected), branch-name input
- [x] After branch-name entry, if it matches an existing local (rendered) branch, skip base-branch question and treat as existing-branch reuse
- [x] Confirmation screen before executing: prints resolved Task / Repository / Base / Branch / Worktree path, prompts `Create worktree? [Y/n]`
- [x] Fully args+flags-specified invocations still skip the confirmation screen entirely (no regression from ticket 02)
- [x] Manual verification of the interactive path end-to-end (huh forms not unit tested per spec §7) — confirmed no-tty path errors cleanly instead of hanging; full interactive walkthrough needs a real terminal to click through by hand
