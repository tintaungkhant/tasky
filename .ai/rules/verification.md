Applies to: all tests and all checking and reporting of a change.

# Verification

- Report only what you verified. Name the checks you ran and their results, and say which ones you skipped or could not run.
- Never make a failing check pass by weakening it: no skipped tests, removed assertions, suppressions, lowered thresholds, or `--no-verify`. Fix failures you caused; report the others. A weakened check hides the bug from everyone after you.
- The checks that must pass are listed under "Checks" in `AGENTS.md`.

## Tests

Write the smallest test for the change first and see it fail for the intended reason, then make it pass. For a bug, reproduce it with a test first. If that is not feasible, say so instead of claiming a failing step occurred.

- Test behavior, not visuals. Each test must catch a distinct defect; do not retest framework guarantees.
- Keep existing tests. Ask before deleting one.

## Visual checks

- Prefer code and proofs that do not depend on a visual check, such as a behavior test or a request against the server.
- Browser and visual checks: see `autonomy.md`. When a change needs a visual check you did not do, tell the user what to open and what to look for.

## Real side effects

During tests and verification, never send real messages, emails or payments, and never touch a non-test database. Use mocks. A real send needs an explicit user request for that specific check.
