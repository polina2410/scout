---
name: code-reviewer
description: >
  Deep code review for TypeScript/React/Next.js code. Use for interactive review of a file,
  PR, or specific logic. Goes deeper than code-scanner agent — focuses on correctness,
  React patterns, security, and maintainability. Trigger words: review this code, review
  this file, deep review, what's wrong with this, is this correct, review my implementation.
argument-hint: <file-or-description>
---

# Code Reviewer

Review: $ARGUMENTS

---

## Process

1. Read the file(s) in full before commenting
2. Evaluate across all six dimensions below
3. Report findings grouped by severity
4. Suggest specific fixes — not just problems

---

## Six Review Dimensions

### 1. Correctness
- Logic matches the intended behavior
- Edge cases handled: empty arrays, null/undefined, 0, empty string
- Async operations properly awaited
- No stale closures in hooks (`useCallback`/`useEffect` dependency arrays)
- No accidental state mutation

### 2. Security
- No secrets or API keys in client-side code
- User input sanitized before use
- No `dangerouslySetInnerHTML` without explicit sanitization
- API routes return `{ error }` with no internal details exposed
- External data validated with Zod before use

### 3. Performance
- No unnecessary re-renders (missing `memo`, unstable object/array literals as props)
- No expensive computations in render — use `useMemo`
- Correct `useEffect` dependencies — no missing or excess deps
- No N+1 fetch patterns
- Large imports done selectively (e.g. `import { x } from 'lib'` not `import lib`)

### 4. Convention Compliance (CLAUDE.md)
- TypeScript — no `any`, no non-null assertions (`!`) without justification
- Named exports only — no default exports
- CSS Modules — no inline styles, no global class strings
- Components are `const` arrow functions
- API calls only in `lib/` — not inline in components or hooks
- No magic numbers — named constants in `constants/`

### 5. Architecture
- Component in the right folder (`features/` vs `UI/`)
- Logic in `lib/` or `hooks/` — not inside components
- No prop drilling beyond 2 levels
- Single responsibility — one thing per file

### 6. Maintainability
- Naming is clear and consistent with the rest of the codebase
- No dead code or commented-out blocks
- No `TODO` comments without a linked issue
- DRY — no copy-pasted logic that could be a shared util or hook
- No overly clever code that requires a comment to understand

---

## Output Format

### Must Fix
Issues that are incorrect, insecure, or violate core conventions.

### Should Fix
Quality issues that will cause problems over time.

### Consider
Minor improvements — optional, left to the developer's judgment.

For each finding:
- **File:** `path/to/file.tsx:42`
- **Issue:** What's wrong
- **Fix:** Specific change to make

End with: **Overall verdict** — Approve / Approve with fixes / Needs rework.

---

## AI-Generated Code

If the code being reviewed was produced by an AI tool, apply extra scrutiny:
- Verify all logic independently — do not assume AI output is correct
- Check for hallucinated APIs, non-existent methods, or plausible-but-wrong patterns
- Confirm tests actually cover the AI-generated logic, not just pass around it
- Flag any code that looks like it was accepted without understanding
