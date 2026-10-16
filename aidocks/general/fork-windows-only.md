---
name: fork-windows-only
description: "The fork stays Windows-only: no GitHub workflows, no Mac/Linux/iOS builds (user decision 2026-10-01)"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-01T21:34:28.915Z
---

The fork (tsatria03/ipatool) stays Windows-only. Its .github folder (upstream workflows, issue templates, funding, lint config) was removed in 689ba1b (2026-09-26). On 2026-10-01 I offered a GitHub Actions workflow to build the CLI for Mac on a macOS runner; the user declined: "I'll keep it windows only."

**Why:** the point of the fork is the accessible Windows GUI (walk is Windows-only); the README already says "This fork supports Windows only."
**How to apply:** don't propose Mac/Linux/iOS builds or CI workflows again. When merging upstream, if upstream edits files under .github/ (a modify/delete conflict), keep them deleted. Build locally into releases\ipatool\ ([[build-output-folder]]).
