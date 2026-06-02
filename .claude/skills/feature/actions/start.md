# Start Action

1. Read current-feature.md - verify Goals are populated
2. If empty, error: "Run /feature load first"
3. Set Status to "In Progress"
4. Create and checkout the feature branch (derive name from H1 heading)
5. List the goals, then implement them one by one

## Quality Gate (run after all goals implemented)

6. Run `pnpm lint` — fix any errors before continuing
7. Run `pnpm test:run` — fix any failing tests before continuing
8. Run `pnpm build` — fix any TypeScript or Next.js compilation errors before continuing
9. Use the `ui-reviewer` agent to check the UI visually
10. If the feature touches interactive elements, forms, or navigation — use the `a11y` agent to audit keyboard navigation and ARIA patterns
11. If the feature adds or modifies any file in `app/api/` — use the `api` agent to verify route design, error shapes, and Zod validation
12. Only report the feature as done when all checks pass