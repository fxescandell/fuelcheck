# Issue tracker: GitHub (fork)

Issues and specs for this repo live as GitHub issues in the **fork**: `fxescandell/fuelcheck`.

Use the `gh` CLI for all operations, and always pass `--repo fxescandell/fuelcheck`.

> **Why the fork and not upstream.** `origin` points at `emanuelarcos/fuelcheck`, the upstream project. The account used by `gh` (`fxescandell`) has only `pull` on upstream — no `push`, no `triage` — so it cannot create labels, assign issues, or manage sub-issues and blocking edges there. The fork is where the tracker lives; `fork` is the matching git remote.

| Remote   | Repository                     | Role                                        |
| -------- | ------------------------------ | ------------------------------------------- |
| `origin` | `emanuelarcos/fuelcheck`        | Upstream. Read-only for us; code contributions go here. |
| `fork`   | `fxescandell/fuelcheck`         | Tracker. Issues, labels, sub-issues, blocking. |

## Conventions

- **Create an issue**: `gh issue create --repo fxescandell/fuelcheck --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --repo fxescandell/fuelcheck --json number,title,body,labels,comments`.
- **List issues**: `gh issue list --repo fxescandell/fuelcheck --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Make an issue a sub-issue of a parent**: `gh issue create --repo fxescandell/fuelcheck --parent <parent> ...`, or `gh issue edit <parent> --repo fxescandell/fuelcheck --add-sub-issue <child>` afterwards (`gh` 2.94+). Older `gh`: `gh api --method POST repos/fxescandell/fuelcheck/issues/<parent>/sub_issues -F sub_issue_id=<child-db-id>` (database id, as in **Blocking** below). Without sub-issues, put `Part of #<parent>` at the top of the child body.
- **Blocking**: GitHub's **native issue dependencies**, the canonical, UI-visible representation. Add an edge with `gh issue create --repo fxescandell/fuelcheck --blocked-by <n>,<n> ...` or `gh issue edit <child> --repo fxescandell/fuelcheck --add-blocked-by <blocker>` (`gh` 2.94+). Older `gh`: `gh api --method POST repos/fxescandell/fuelcheck/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/fxescandell/fuelcheck/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Count open blockers**: `gh api repos/fxescandell/fuelcheck/issues/<n> --jq .issue_dependencies_summary.blocked_by` (open blockers only; 0 means startable). `gh issue view <n> --repo fxescandell/fuelcheck --json blockedBy` also lists closed blockers, so filter it: `--jq '[.blockedBy.nodes[] | select(.state == "OPEN")] | length'`.
- **Comment on an issue**: `gh issue comment <number> --repo fxescandell/fuelcheck --body "..."`.
- **Apply / remove labels**: `gh issue edit <number> --repo fxescandell/fuelcheck --add-label "..."` / `--remove-label "..."`.
- **Close**: post the explanation with `gh issue comment <number> --repo fxescandell/fuelcheck --body "..."`, then `gh issue close <number> --repo fxescandell/fuelcheck`. On an already-closed issue, `gh issue close --comment` drops the comment and still exits 0.

## Labels

Triage labels (`bug`, `enhancement`, `documentation`, …) are inherited from upstream and are not used by the wayfinding flow. The wayfinding labels are `wayfinder:map`, `wayfinder:research`, `wayfinder:prototype`, `wayfinder:grilling`, `wayfinder:task` — these are the **only** labels a map and its tickets carry.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## When a skill says "publish to the issue tracker"

Create a GitHub issue in `fxescandell/fuelcheck`.

## When a skill says "fetch the relevant ticket"

Read it as in **Read an issue** above.

## Wayfinding operations

Used by the `fx-wayfinder` skill. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --repo fxescandell/fuelcheck --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (see **Make an issue a sub-issue of a parent**). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: see **Blocking** under Conventions.
- **Frontier query**: list the map's open children (`gh issue list --state open --repo fxescandell/fuelcheck`, scoped to the map's sub-issues / task list), drop any with an open blocker (see **Count open blockers**, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --repo fxescandell/fuelcheck --add-assignee @me`, the session's first write.
- **Resolve**: `gh issue comment <n> --repo fxescandell/fuelcheck --body "<answer>"`, then `gh issue close <n> --repo fxescandell/fuelcheck`, then append a context pointer (gist + link) to the map's Decisions-so-far.
