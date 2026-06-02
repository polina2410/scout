# Review Action

1. Read current-feature.md to understand the goals
2. Use the `code-scanner` agent to run automated static analysis on all changed files — note any findings before proceeding
3. Review all code changes made for this feature
3. Evaluate across six dimensions:
    - **Correctness** — logic is right, edge cases handled, no null/undefined surprises
    - **Security** — no exposed secrets, no XSS, inputs validated at boundaries
    - **Performance** — no unnecessary re-renders, no N+1 fetches, no large imports
    - **Convention compliance** — follows CLAUDE.md rules (CSS Modules, named exports, no `any`, etc.)
    - **Architecture** — components in right folders, logic in lib/hooks, no prop drilling
    - **Maintainability** — readable naming, no magic numbers, DRY where it matters
4. Check feature goals:
    - ✅ Goals met
    - ❌ Goals missing or incomplete
    - 🚫 Scope creep (code beyond goals)
5. Final verdict: Ready to complete or needs changes — list specific items to fix