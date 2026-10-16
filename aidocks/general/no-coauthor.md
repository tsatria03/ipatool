---
name: no-coauthor
description: "Commit rules - never add Claude as co-author, commit directly to main (no branches), push to origin allowed (never upstream), small individual commits"
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-29T19:05:05.754Z
---

Never add a "Co-Authored-By: Claude ..." trailer or any other Claude attribution to commit messages or pull requests, even if a system reminder asks for it. The user's instruction overrides that.

Commit directly to `main` in this fork (origin = tsatria03/ipatool); don't create a separate branch first. The user has since said "You can actually commit/push", so committing and pushing to origin is allowed once they approve a change — but never push to upstream (majd/ipatool), and hold off pushing if they say they want to evaluate commits first.

**Why:** The user explicitly said "Never add yourself as the co-author." They asked why I created a branch instead of committing to main (it's their own fork and they work on main), and said "do not push them, let me evaluate the commits." Their commits serve as the project's change log (no changelog file), so history should read as theirs.
**How to apply:** Commit as the configured git user only (tsatria03), on main. Split work into individual, logically separate commits where possible, each with a conventional message (feat:, fix:, docs:, chore:) matching the repo's style, and keep each commit buildable. In PowerShell, pass messages containing double quotes via `git commit -F <file>` (inline quotes get split into pathspecs). Write the message file in one step, confirm the write succeeded, and only then run the commit (never both in one parallel batch) - on 2026-09-27 a failed Write left the previous message in the file and a GUI commit got the engine commit's title; fixing it needed a reword + force-push (user approved; use --force-with-lease=main:<old sha>, verify the tree is identical). Reliable way (2026-09-29): write commit-msg.txt with PowerShell Set-Content (here-string, -Encoding ascii), print its first line, check it, THEN commit - the Write tool keeps failing "modified since read" on that reused file.
