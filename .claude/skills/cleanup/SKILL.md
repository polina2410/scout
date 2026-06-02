---
name: cleanup
description: Clean up project housekeeping tasks (add "run" to execute fixes)
argument-hint: run|check
---

Review the codebase for cleanup tasks:

1. Make sure that the history in @context/features/features-history.md is in order from oldest to newest
2. Find unnecessary console.log statements
3. Find unused imports
4. Check for stale TODO comments
5. Find orphaned/unused files
6. Check that context files match actual project state
7. Find `@ts-ignore` comments that might be stale
8. Verify `.gitignore` covers all sensitive and generated files:
   - `.env`, `.env.local`, `.env*.local` — secrets must never be committed
   - `coverage/` — test coverage output
   - `/.next/` — Next.js build output
   - `/node_modules` — dependencies
   - Any new secret or generated file patterns introduced since last check

**Mode: $ARGUMENTS**

If no argument or argument is "check":

- Only report findings, don't modify anything
- List what WOULD be cleaned up

If the argument is "run" or "fix":

- First, report all findings with numbered items
- Then ask: "Which items would you like me to fix? (enter numbers like 1,3,5 or 'all' or 'none')"
- Wait for user response before making any changes
- Only fix the items the user specifies
- Report what you changed