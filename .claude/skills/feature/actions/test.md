# Test Action

1. Read current-feature.md to understand what was implemented
2. Use the `coverage` agent to identify which files added in this feature have no test coverage
3. For uncovered functions with testable logic, use the `test-master` skill to write unit tests:
    - Focus on server actions and utilities (not components)
    - Test happy path and error cases
    - Do not write tests just to write them. Use your best judgment
4. Run `pnpm test:run` to verify all tests pass
5. Run `pnpm test:coverage` and confirm no threshold is newly failing