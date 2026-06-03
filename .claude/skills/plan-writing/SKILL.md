---
name: plan-writing
description: >
  Write a feature spec before implementation starts. Reads PLAN.md and existing specs
  for context, produces a detailed spec in the project format, and saves it to context/specs/.
  Trigger words: spec, create spec, write spec, spec for step, let's plan, before we start.
argument-hint: <step-name-or-description>
---

# Spec Writing

Write a feature spec before implementation starts.

## Task

Write a spec for: $ARGUMENTS

---

## Rules

- Match the format of existing specs in `context/specs/` exactly
- Save to `context/specs/{NN}-{slug}.md` — use the next available number
- Numbered `##` sections, one per logical concern
- `## Acceptance criteria` at the end with checkboxes
- Include a `## Carry-forward items` section if prior review found deferred issues
- **No checkbox task lists** — specs describe *what* to build, not *how to track progress*

---

## Process

1. Read `PLAN.md` to understand where this step fits in the overall build
2. Read existing specs in `context/specs/` to match format and numbering
3. Read `CLAUDE.md` for project constraints, `openapi.yaml` for API contract if relevant
4. Read any existing stub files for the packages being specified
5. Read `context/features/features-history.md` to understand what's already built
6. Write the spec — numbered sections, concrete signatures/SQL/types where needed
7. Save to `context/specs/{NN}-{slug}.md`
8. Show a summary and ask for approval

---

## Spec Format

```markdown
# Spec {N} — {Name}

**Plan ref:** Phase X, Step Y
**Goal:** One sentence describing the end state.

---

## 1. {First concern}

{Detailed description, code signatures, SQL, types, behaviour rules}

---

## 2. {Second concern}

...

---

## Carry-forward items (if any)

Issues caught in a prior review that belong in this step:

- **Item** — description and fix

---

## Acceptance criteria

- [ ] {Concrete, verifiable check}
- [ ] {Another check}
```

---

## Good vs Bad Sections

| Bad (vague) | Good (concrete) |
|---|---|
| Add the client | Show the struct, `New` signature, and exact error behaviour |
| Write tests | Name each test case and what it asserts |
| Handle errors | Specify which errors map to which HTTP status codes |
| Wire it up | Show the exact `main.go` snippet and what to defer/close |