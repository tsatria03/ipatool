---
name: plan-my-apps-100-per-page
description: "Plan (2026-09-26, requested by the user, in progress) to show 100 apps per page in My apps instead of 25"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T05:49:32.988Z
---

**Status:** COMPLETED 2026-09-26 (myapps100-test.ps1: 100 rows, "Page 1 of 36", last page 54, search 13 rows). User asked 2026-09-26 ("I wish the list showed 100 apps at a time ... when you are not searching my apps"). Interpretation: 100 per page for every My apps view (search results rarely exceed 100; keeps paging consistent) — told the user; switch search back to 25 if they object. Builds on [[plan-my-apps-filter-reload]]. Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]].

## Design
- gui/purchases.go: `appsPerPage` 25 -> 100 (used by showMyApps via pageOf). Nothing else depends on it (the engine request already fetches everything).
- Page field becomes e.g. "Page 1 of 36, 3554 apps"; last page has 54.
- README Features: "25 at a time" -> "100 at a time". myapps_test.go: add a 100-per-page case (36 pages, last 54).

## Steps
1. [ ] Change constant, tests, README.
2. [ ] Hidden-desktop test: first load shows 100 rows and "Page 1 of 36, 3554 apps"; Next page -> page 2 starts at row 101's app; last page has 54 rows; search still works.
3. [ ] Commit, push.
