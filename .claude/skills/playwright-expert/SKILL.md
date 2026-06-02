---
name: playwright-expert
description: >
  E2E testing specialist using Playwright MCP. Use when writing browser automation tests,
  debugging UI interactions, or validating user flows end-to-end. Follows Page Object Model,
  role-based selectors, and auto-waiting. Trigger words: E2E test, browser test, playwright,
  test the user flow, automate this interaction, visual regression.
argument-hint: <flow-or-page-to-test>
---

# Playwright Expert

Write or debug E2E tests for: $ARGUMENTS

---

## Setup

- Dev server must be running: `pnpm dev` (http://localhost:3000)
- Use `mcp__playwright__*` tools for browser interaction
- Tests go in `__tests__/e2e/` if writing to files

---

## Process

1. Navigate to the page under test
2. Take a snapshot to understand current DOM state
3. Identify interactive elements using role-based selectors
4. Write the test flow: navigate → interact → assert
5. Verify the flow passes in a clean browser state

---

## Selector Priority (highest to lowest)

```ts
// 1. Role-based (preferred)
page.getByRole('button', { name: 'Start Quiz' })
page.getByRole('heading', { name: 'World Explorer' })

// 2. Label / placeholder
page.getByLabel('Search countries')
page.getByPlaceholder('Search...')

// 3. Text content
page.getByText('Next question')

// 4. Test ID (when nothing else works)
page.getByTestId('flag-mosaic')

// Never: CSS classes, nth(), first() without justification
```

---

## Core Patterns

### Basic Flow
```ts
await page.goto('http://localhost:3000');
await page.getByRole('button', { name: 'Start Quiz' }).click();
await expect(page.getByRole('heading', { name: 'Quiz Setup' })).toBeVisible();
```

### Waiting (use auto-waiting — never waitForTimeout)
```ts
// Good — waits for element to appear
await expect(page.getByText('Score:')).toBeVisible();

// Bad — arbitrary sleep
await page.waitForTimeout(2000); // ❌ never do this
```

### Assertions
```ts
await expect(page.getByRole('button')).toBeEnabled();
await expect(page.getByText('10/10')).toBeVisible();
await expect(page).toHaveURL('/results');
```

---

## Best Practices

**Do:**
- One user flow per test — keep tests focused
- Reset to clean state between tests (navigate to home)
- Use `expect().toBeVisible()` not `isVisible()` — auto-waits
- Take screenshots on failure for debugging

**Don't:**
- Share browser state between tests (login once, use everywhere) — causes flaky tests
- Use `nth()` or `first()` selectors — fragile
- Hardcode timeouts — trust Playwright's auto-wait
- Test implementation details — test what the user sees

---

## Playwright MCP Tools Available

| Tool | Use for |
|------|---------|
| `mcp__playwright__browser_navigate` | Go to a URL |
| `mcp__playwright__browser_snapshot` | See current DOM state |
| `mcp__playwright__browser_click` | Click an element |
| `mcp__playwright__browser_type` | Type into an input |
| `mcp__playwright__browser_take_screenshot` | Capture visual state |
| `mcp__playwright__browser_wait_for` | Wait for condition |
| `mcp__playwright__browser_evaluate` | Run JS in browser |
| `mcp__playwright__browser_network_requests` | Inspect API calls |

---

## Key User Flows to Test in This Project

<!-- Add project-specific flows here. Examples:
- Home page loads with core content visible
- Primary CTA navigates to the correct page
- Main feature happy path (start → complete)
- Form submission shows success/error feedback
- Navigation between main routes works correctly
-->
