---
name: plans-in-memory-first
description: "Before building any agreed plan, write it to memory first, then re-read and review that memory before starting work"
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T01:25:57.565Z
---

For any plan the user agrees to, first write the plan as a memory file (a `project` memory, linked from MEMORY.md), then read it back and review it against the code before building anything. Keep the plan file updated as steps finish or change.

**Why:** The user asked for this as a standing rule ("Always write memory plans first, then review what's in it before building the plan") after agreeing to the self-contained GUI plan ([[plan-self-contained-gui]]). It keeps plans durable across sessions and makes sure work follows what was agreed.
**How to apply:** Plan agreed → Write `plan-<topic>.md` with goal, design, steps, risks and a status line per step → add it to MEMORY.md → Read it back, check each assumption in the code, fix the file → tell the user the plan is recorded and reviewed → only then build, updating step status as you go.
