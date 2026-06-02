---
name: research
description: Run a research task to generate documentation
argument-hint: <prompt-name>
---

## Task

Execute research task: $ARGUMENTS

---

### Instructions

1. If no argument provided, error: "Usage: /research <prompt-name>"
2. Look for prompt file at `context/research/{$ARGUMENTS}.md`
3. If not found, error: "Prompt file not found at context/research/{$ARGUMENTS}.md"
4. Read the prompt file which should contain:
    - **Output**: Where to write results (e.g., `context/content-types.md`)
    - **Research**: What to investigate
    - **Include**: Specific details to capture
    - **Sources**: What files/tools to use
5. Execute the research using appropriate tools:
    - Read files (schemas, constants, components)
    - Search codebase for patterns
    - Use Context7 MCP to fetch documentation when research involves a library, framework, or external API — prefer a version-specific library ID (e.g. `/org/project@18`) if the project pins a specific version, to avoid mismatches with newer APIs
6. Write findings to the specified output location
7. Summarize what was discovered

---

### Rules

- This command produces DOCUMENTATION only
- Do NOT modify source code files
- Do NOT create branches or commits
- Output should go to `/docs/` unless otherwise specified
- Use subagents for thorough exploration if needed