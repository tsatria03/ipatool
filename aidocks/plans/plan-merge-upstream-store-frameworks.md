---
name: plan-merge-upstream-store-frameworks
description: Merge upstream 5dc4c80 (Apple store frameworks rewrite) and d9c0247 (compress bump); fix gui/go.mod's dead lzfse replace, README note, rebuild; do not push
metadata:
  type: project
---

**Status:** IN PROGRESS (agreed 2026-10-31; user: "Do not push yet").

**Goal:** bring upstream majd/ipatool up to d9c0247 into main. A trial merge in a scratch worktree on 2026-10-31 built both programs and passed all engine and GUI tests.

**Background:** 5dc4c80 makes the engine emulate Apple's store frameworks (internal/store, a precomputed profile) and deletes internal/lzfsecompat and the lzfse-cgo dependency. NewAppStore keeps its signature, so gui/backend.go needs no change; cmd/ is untouched, so the shared-login copy stays in sync. On first use the engine downloads and caches more of Apple's public restore image (updates.cdn-apple.com, no account requests, see [[protect-apple-account]]).

**Steps:**
1. Merge upstream/main with --no-commit. Conflicts: .github/workflows/*.yml (keep deleted, [[fork-windows-only]]); .gitignore (keep the fork's lines, take upstream's new .claude line). Status: pending
2. gui/go.mod: remove the replace line for lzfse-cgo => ../internal/lzfsecompat, go mod tidy. Include in the merge commit so it builds. Status: pending
3. Build both programs and run tests (engine tests need go generate ./... first, because *_mock.go files are gitignored). Commit the merge with the usual message "Merge remote-tracking branch 'upstream/main'". Status: pending
4. Separate docs commit: drop the lzfse replacement sentence from README's "Shared login" paragraph. Status: pending
5. Rebuild ipatool.exe and ipatool-gui.exe into releases\ipatool ([[build-output-folder]]). Status: pending
6. Do NOT push; the user evaluates the commits first ([[no-coauthor]]). Status: pending
