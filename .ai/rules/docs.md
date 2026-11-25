Applies to: every change, including code, rules and decisions.

# Docs

- Keep docs true. When a change affects what the docs describe (behavior, data model, API contracts, setup, architecture), update them in the same change. Example: a table rename with a migration needs a doc update; a label text change does not.
- Each topic has one home. Link to it; do not repeat it.
- Before you finish, check the docs for anything your change made wrong.
- When a doc is no longer true, update it or delete it. History lives in `git log` and closed tickets.
- Do not create per-task summary or diary files.
- When the user settles a decision, add it under "Decisions" in `AGENTS.md`. When a decision changes, strike the old line through and add the date.
- When the project gains generated files, list them under "Generated files" in `AGENTS.md` with the command that regenerates them.
