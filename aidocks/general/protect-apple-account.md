---
name: protect-apple-account
description: "Avoid bulk signed-in requests to Apple with the user's account; public lookups in small batches are fine"
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-28T03:51:41.376Z
---

Don't design features or tests that send many signed-in (account) requests to Apple's store servers, such as per-app download-info or size requests for hundreds of apps.

**Why:** The user agreed (2026-09-27) they "wouldn't want the apple servers to be overloaded with that many requests. That could affect the use of my own account." (context: measuring exact sizes of ~961 removed apps would need ~961 authenticated requests.)
2026-09-27 the user also declined batch downloads for the same reason ("it would probably send too many requests if I check multiple apps") - don't propose batch/queued downloads again.
**How to apply:** Prefer the public, unauthenticated iTunes lookup/search API in batches (150 IDs, a few at a time) for bulk data; keep account requests to what the user explicitly does (sign in, one download, one version list). Hidden-desktop tests: at most a handful of real downloads, small apps only. See [[apple-account-us]].
