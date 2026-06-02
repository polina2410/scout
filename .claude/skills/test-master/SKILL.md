---
name: test-master
description: >
  QA specialist for writing Vitest unit and integration tests. Use when adding tests for
  hooks, utilities, API routes, or context providers. Enforces happy path + error cases,
  descriptive names, no shared state. Trigger words: write tests, add tests, test coverage,
  test this hook, test this function, missing tests.
argument-hint: <file-or-function-to-test>
---

# Test Master

Write thorough, maintainable Vitest tests for: $ARGUMENTS

---

## Stack

- **Framework:** Vitest
- **Component testing:** @testing-library/react
- **Hook testing:** `renderHook` + `act` from @testing-library/react
- **Setup file:** `__tests__/setup.ts`
- **Config:** `vitest.config.mts`
- **Location:** `__tests__/` mirroring project structure

---

## Process

1. Read the function/hook/module to test — understand inputs, outputs, and side effects
2. Identify all meaningful test cases (see categories below)
3. Check `__tests__/` for existing tests to avoid duplication
4. Write tests following the standards below
5. Run `pnpm test:run` — fix any failures before reporting done

---

## What to Test

### Custom Hooks
- Happy path: correct return values given valid input
- State transitions: values update correctly after actions
- Edge cases: empty arrays, null/undefined inputs, boundary values
- Use `renderHook` + `act` for state updates

### Utility Functions (`lib/`, `utils/`)
- Every branch of logic (if/else, switch)
- Invalid inputs and error throws
- Pure function: same input → same output

### API Route Handlers (`app/api/`)
- Successful response shape matches schema
- Error response returns `{ error: string }` with correct status
- Mock `fetch` — never call real external APIs in tests

### Zod Schemas (`schemas/`)
- Valid data parses successfully
- Invalid/missing fields throw or return errors
- Optional fields behave correctly when absent

---

## Standards

```ts
// Descriptive names: "should {behaviour} when {condition}"
it('should return empty array when no countries match the filter', () => { ... })

// Arrange → Act → Assert structure
it('should increment count on click', () => {
  // Arrange
  const { result } = renderHook(() => useCounter());
  // Act
  act(() => result.current.increment());
  // Assert
  expect(result.current.count).toBe(1);
});
```

**Always:**
- Test both success and failure paths
- Mock external dependencies (`fetch`, timers, modules)
- Use `beforeEach` to reset state — never share mutable state between tests
- Assert on specific values, not just truthiness

**Never:**
- `waitForTimeout()` or arbitrary sleeps — use `waitFor` from Testing Library
- Call real external APIs
- Leave `console.log` in tests
- Write tests just to hit coverage numbers — test meaningful behaviour

---

## Mocking Patterns

```ts
// Mock fetch
global.fetch = vi.fn().mockResolvedValue({
  ok: true,
  json: async () => mockData,
});

// Mock a module
vi.mock('@/lib/fetchCountries', () => ({
  fetchCountries: vi.fn().mockResolvedValue(mockCountries),
}));

// Reset mocks between tests
beforeEach(() => vi.clearAllMocks());
```

---

## Run Tests

```bash
pnpm test:run                                                    # all tests
pnpm exec vitest run __tests__/hooks/useQuiz.test.ts             # single file
```
