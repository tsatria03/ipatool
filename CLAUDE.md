# CLAUDE.md

This file points Claude to the project's memories. The memories themselves live in the `aidocks` folder; read them before working on this repository.

## Where memories live

- `aidocks/general/` holds general information: who the user is, how they want Claude to work, account and safety rules, build and commit rules, the feature ideas backlog.
- `aidocks/plans/` holds plans, finished and unfinished: one file per feature or fix, with its status, steps, test results and commits.
- `aidocks/private/` holds personal information, such as the user's App Store account and purchase details. It's listed in `.gitignore`, so it never goes to GitHub; read it like the others.
- `aidocks/general/MEMORY.md` is the index: one line per memory file, linking to it. **Read it first at the start of every session**, then open the files that apply to the task.

## Rules for memories

- **Use `aidocks` for every new memory**, not Claude's built-in memory folder. (Since 2026-10-16 the built-in folder only holds a pointer to this file.)
- Put general facts and rules in `aidocks/general/`, plans in `aidocks/plans/` (file names start with `plan-`), and anything personal in `aidocks/private/`.
- **The user's Apple ID (email address or phone number) and device names are only ever written in `aidocks/private/`** (the user asked for this on 2026-10-16). Everywhere else, write "the user's Apple ID" or "the user's iPhone".
- **Keep `general` and `plans` free of private details**, because they're published with the repository: no names, user names in paths (write `%USERPROFILE%`, `<repo>` or `<scratchpad>`), email addresses, device names or IDs, Wi-Fi addresses or network names, order numbers, or purchase history. Those go in `aidocks/private/`, or are left out. Exception: the user is happy for their first name to appear in made-up example app titles, such as "Tony's Game" in the README and tests.
- Each memory is one Markdown file about one thing, with this front matter:

  ```markdown
  ---
  name: short-kebab-case-slug
  description: one-line summary, used to decide whether the file is relevant
  metadata:
    type: user | feedback | project | reference
  ---
  ```

  For rules and decisions, add **Why:** and **How to apply:** lines. Link related memories with `[[their-name]]`.
- After adding a memory, add one line for it to `aidocks/general/MEMORY.md`: `- [Title](file.md) — short hook`. Plans are linked as `../plans/file.md`. Keep the index short; the details belong in the files.
- Before adding a memory, check whether an existing file already covers it, and update that file instead. Fix or remove memories that turn out to be wrong.
- Write agreed plans into `aidocks/plans/` **before** building, re-read and review them, then build (see `aidocks/plans/plans-in-memory-first.md`). Update a plan's status as steps finish.
- Don't store what the repository already records (code structure, git history), and never store passwords, passphrases or two-factor codes.

## Most important rules (details in the files)

- The user uses the NVDA screen reader: every GUI change must be accessible (`screen-reader-user.md`, `no-control-punctuation.md`, `lists-first-letter-nav.md`).
- Ask one question per reply, as plain text, never with a picker (`no-pickers.md`).
- Protect the user's Apple Account: no bulk signed-in requests to Apple (`protect-apple-account.md`).
- GUI tests run on a hidden desktop only (`gui-tests-hidden.md`).
- Commits: straight to `main`, small and separate, never a co-author line; push to `origin` only, never upstream (`no-coauthor.md`).
- The fork is Windows-only (`fork-windows-only.md`); programs are built into `releases\ipatool` (`build-output-folder.md`).
