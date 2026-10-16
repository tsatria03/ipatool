---
name: build-output-folder
description: Build ipatool.exe and ipatool-gui.exe into releases\ipatool\ (not the releases root)
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-14T21:34:19.883Z
---

When building the programs for the user, put both in `releases\ipatool\` inside the repo:
- `go build -o releases\ipatool\ipatool.exe` (repo root)
- `cd gui` then `go build -ldflags=-H=windowsgui -o ..\releases\ipatool\ipatool-gui.exe .`

HOLD ENDED 2026-10-14: all 3 plans built, both exes rebuilt into releases\ipatool. (Was: user said "Do not compile the ipa tool into an exe until all 3 plans are built" - no releases\ipatool builds until [[plan-apple-id-check]], the "Not signed for your iPhone" message and Find App ID are all built (see [[feature-ideas-backlog]]). Scratchpad test builds for hidden tests are still OK.

**Why:** User, 2026-09-29: "Please put the ipatool programs inside here from now on" (they created the folder, apparently to hand the programs to someone). releases/ is gitignored (f961b83).
**How to apply:** Rebuild there after GUI/engine changes when the user wants current builds; don't leave copies in the releases root. README's build commands were updated to releases\ipatool\ too (8df9148, user asked).
