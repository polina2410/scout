# Custom Subagents

Custom Claude Code subagents for this project. These files go in your project's `.claude/agents/` folder.

- `code-scanner.md` - Scans the codebase for security, performance and code quality issues
- `refactor-scanner.md` - Identifies code that can be broken up or refactored
- `ui-reviewer.md` - Reviews the UI/UX and provides feedback on design and usability
- `debugger.md` - Root-cause analysis specialist for TypeScript/React bugs
- `developer.md` - Implements features following project conventions with a quality gate
- `docs-writer.md` - Creates and updates technical documentation without modifying source code
- `react-refactoring-expert.md` - Refactors and reduces complexity in React/TypeScript code without adding features
- `devil.md` - Planning-phase devil's advocate that challenges requirements and architecture decisions
- `ba.md` - Business analyst for requirements, user stories, and acceptance criteria before implementation
- `qa.md` - E2E and integration testing specialist using Playwright MCP; covers user journeys, visual regression, and accessibility
- `devops.md` - CI/CD and deployment specialist; owns GitHub Actions workflows, environment variables, Vercel config, and secret hygiene
- `a11y.md` - Accessibility specialist; WCAG AA audits, keyboard navigation, ARIA patterns, focus management, live region announcements
- `api.md` - API route auditor; checks thin routes, Zod validation, error response shapes, caching, rate limiting coverage
- `coverage.md` - Test coverage auditor; runs coverage analysis, identifies gaps by priority, hands off to test-master
- `dependency-manager.md` - pnpm dependency hygiene; outdated packages, security audits, upgrade planning, new package evaluation