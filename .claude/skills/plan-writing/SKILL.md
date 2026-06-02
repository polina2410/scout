---
name: plan-writing
description: >
  Structured task decomposition before implementation. Use before starting any non-trivial
  feature, refactor, or bug fix. Produces a concise, verifiable plan saved to context/plans/.
  Trigger words: plan, let's plan, before we start, design the approach, break this down.
argument-hint: <task-description>
---

# Plan Writing

Create a concise implementation plan before starting work.

## Task

Write a plan for: $ARGUMENTS

---

## Rules

- **Max 1 page** — if it's longer, simplify
- **5–10 tasks** — no more, no fewer
- Each task must have a **concrete action** and a **verifiable outcome**
- Save the plan to `context/plans/{task-slug}.md`
- Update checkboxes as work progresses

---

## Process

1. Understand the goal — read `context/features/current-feature.md` if a feature is active
2. Identify affected files (read them before writing the plan)
3. Break work into 5–10 sequential tasks
4. Write the plan using the format below
5. Save to `context/plans/{task-slug}.md`
6. Show a summary and ask for approval before starting

---

## Plan Format

```markdown
# Plan: {Task Name}

**Goal:** One sentence describing the end state.

## Tasks

- [ ] 1. {Concrete action} → verify: {how to confirm it worked}
- [ ] 2. {Concrete action} → verify: {how to confirm it worked}
- [ ] ...

## Done When

- [ ] `pnpm lint` passes
- [ ] `pnpm test:run` passes
- [ ] `pnpm build` passes
- [ ] {feature-specific check}

## Notes

{Any constraints, risks, or decisions to flag}
```

---

## Good vs Bad Tasks

| Bad (vague) | Good (concrete) |
|-------------|-----------------|
| Set up the component | Create `components/features/X/X.tsx` with named export `X` |
| Add tests | Write test for `useX` covering happy path and empty-state edge case in `__tests__/hooks/useX.test.ts` |
| Fix the bug | Trace null value in `[function]` → add guard before `.map()` call |
| Make it work | Run `pnpm dev`, navigate to `[route]`, confirm [behaviour] |
