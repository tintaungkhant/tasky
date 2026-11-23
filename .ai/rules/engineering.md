Applies to: all code in this repo.

# Engineering

- Think as a domain expert in what the project is. Before building, apply how that kind of product works: its terms, its usual flows, and the issues it commonly hits. Example: in a CRM, know leads, contacts, deals and pipelines, and expect duplicate contacts and ownership conflicts. Use the domain's terms in code and UI.
- Make the code light enough to run well on modest hardware: no needless memory, queries, background processes or services. Size it for this project's real load; do not cut needed features or capacity to fit a tiny server.
- Build only what the task needs. Write the least code that solves it correctly: lazy means less code, not less care. Required safety and integrity are current requirements, not future work.
- Extract shared code once a second real use exists. Similar syntax does not prove shared meaning. Give each business rule one implementation, because a duplicated rule drifts.
- Before writing non-trivial logic, check the framework, then existing dependencies, then a popular, maintained package. When you hand-roll something a package could do, say why in one line. Install permission is in `autonomy.md`.
- Your training data may be outdated. Before using a fast-moving library, read the docs shipped with the installed version (for example `node_modules/<pkg>/dist/docs/`). Trust the repo's docs and `git log` over memory.
- Create new files with the framework's generator (for example `php artisan make:model --no-interaction`), then edit them. Generator stubs follow the framework's current conventions.
- Write code a junior developer can read and change. Prefer plain, direct code over clever patterns: add an interface, contract, trait, base class or layer only when the code needs it now, not to look senior. Plain does not mean copy-paste: shared logic still follows the extraction rule above.
- Follow SOLID, clean code, DRY, YAGNI and the standard design patterns. Use them to make code simpler to read and change. When applying one would make the code harder for a junior to follow, choose the simpler code.
- Make code readable through clear names and small functions, not through comments. Comment only what the code cannot say: why a non-obvious choice was made, a workaround, a gotcha. Keep existing comments unless they are wrong.
- Validate every input on the server. Treat every endpoint and action as publicly callable. Never trust values the client derives, such as prices, totals or tax: compute them on the server.
