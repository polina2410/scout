---
name: api-design
description: >
  API design principles for Next.js App Router routes. Activate when adding or modifying
  API routes, fetch logic, or Zod schemas. Covers route structure, error handling,
  validation, and response shape conventions.
argument-hint: review|new
---

# API Design Principles

Guidance for designing and reviewing Next.js App Router API routes in this project.

## When to Use

- Adding a new route under `app/api/`
- Modifying `lib/fetchCountries.ts` or adding new fetch functions
- Changing Zod schemas in `schemas/`
- Reviewing an existing route for correctness

## Task

$ARGUMENTS

| Action | Description |
|--------|-------------|
| `review` | Audit an existing route against these principles |
| `new` | Design a new route — walk through the checklist below |

---

## Core Principles

### 1. Route Structure

- One file per resource: `app/api/{resource}/route.ts`
- Export named HTTP method handlers only: `GET`, `POST`, `PATCH`, `DELETE`
- Never put fetch logic or business logic inside the route file — delegate to `lib/`
- Route files are thin: receive request → call lib → return response

**Pattern from this codebase:**
```ts
// app/api/countries/route.ts
export async function GET() {
  try {
    const data = await fetchSomething();
    return Response.json(data);
  } catch (err) {
    logger.error('GET /api/resource failed', { error: String(err) });
    return Response.json({ error: 'Internal server error' }, { status: 500 });
  }
}
```

### 2. Fetch / Data Layer (`lib/`)

- All external API calls live in `lib/` — never in route files or components
- Always set a timeout with `AbortController`
- Validate external responses with Zod before mapping — never trust external shapes
- Map external data to your internal `CountryResponse`-style shape before returning
- Throw descriptive errors with the source named: `REST Countries network error: ...`

**Pattern from this codebase:**
```ts
// lib/fetchSomething.ts
const controller = new AbortController();
const timeoutId = setTimeout(() => controller.abort(), 10_000);
try {
  const res = await fetch(URL, { signal: controller.signal, ... });
  if (!res.ok) throw new Error(`Source API failed: ${res.status}`);
  const raw = ExternalSchema.parse(await res.json()); // validate raw
  return raw.map(toInternalShape);                    // map to internal shape
} finally {
  clearTimeout(timeoutId);
}
```

### 3. Zod Validation (`schemas/`)

- Define two schemas per resource: external shape + internal (response) shape
- External schema: mirrors what the third-party API returns (use `.optional()` liberally)
- Internal schema: the clean shape your UI consumes (stricter, no optionals where avoidable)
- Validate at both ends: raw response AND mapped output
- Export schemas and infer types from them — don't duplicate type definitions

```ts
export const ExternalSchema = z.object({ ... });       // what the API returns
export const ResponseSchema = z.object({ ... });        // what the UI receives
export type ExternalData = z.infer<typeof ExternalSchema>;
export type ResponseData = z.infer<typeof ResponseSchema>;
```

### 4. Error Handling

- Route files always wrap in try/catch and return `{ error: 'Internal server error' }` with status 500
- Never expose internal error details to the client
- Log errors server-side with context: route name + error string
- Fetch functions throw — they do not return null or undefined on failure

### 5. Response Shape

- Always return JSON: `Response.json(data)` or `Response.json({ error }, { status })`
- Success: return the data directly (array or object)
- Error: always `{ error: string }` with an appropriate HTTP status code
- Never return different shapes for the same route depending on conditions

### 6. Caching

- Use `next: { revalidate: seconds }` for data that changes infrequently
- Define cache durations as named constants, not magic numbers:
  ```ts
  const SIX_MONTHS_IN_SECONDS = 60 * 60 * 24 * 180;
  ```

---

## Checklist for New Routes

- [ ] Route file in `app/api/{resource}/route.ts`
- [ ] Fetch logic in `lib/fetch{Resource}.ts`
- [ ] External Zod schema defined in `schemas/{resource}.ts`
- [ ] Internal response schema defined and validated after mapping
- [ ] AbortController timeout on fetch
- [ ] Error thrown with named source in message
- [ ] Route handler wrapped in try/catch
- [ ] No internal error details in response body
- [ ] Cache strategy set if data is stable
- [ ] Named constant for any numeric duration or limit

---

## What NOT to Do

- Do not call external APIs directly from components
- Do not return raw external API shapes to the UI — always map
- Do not skip Zod validation on external responses
- Do not use magic numbers for timeouts or cache durations
- Do not put multiple resources in one route file
