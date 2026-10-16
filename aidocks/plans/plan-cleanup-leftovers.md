---
name: plan-cleanup-leftovers
description: "Plan (2026-09-27, approved) - remove unused backend.ownedApps/ownedPage, fillList's unused bool, update App comment in settings.go"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T07:40:32.152Z
---

**Status:** COMPLETED 2026-09-27 (78a27ea, pushed). vet, tests and build passed. Found by the post-compaction rescan. Follow [[plans-in-memory-first]], [[no-coauthor]].

## Steps
1. [ ] gui/backend.go: delete ownedPage + ownedApps (My apps uses ownedAppsAll only).
2. [ ] gui/search.go: fillList returns nothing.
3. [ ] gui/settings.go: App comment mentions size (search results have price and size; purchase history has purchase date, neither price nor size).
4. [ ] gofmt, vet, go test, build; one commit "refactor(gui): remove leftovers"; push. No behaviour change, so no hidden-desktop test needed beyond the build.
