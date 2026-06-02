# Current Feature: Project Structure & Tooling

## Status

In Progress

## Goals

- `backend/` Go module initialized with correct module path and internal package placeholders
- `frontend/` Vite + React 19 + TypeScript scaffolded with all required dependencies installed
- Feature-based folder structure in place: `features/gallery/`, `features/filters/`, `features/map/`, `store/`, `api/generated/`
- `vite.config.ts` configured with dev proxy, jsdom test environment, and test setup file
- `tsconfig.json` has strict mode, `noImplicitAny`, `noUncheckedIndexedAccess`
- `openapi-typescript` codegen wired up as `pnpm generate` script
- Makefile with `dev`, `build`, `test`, `seed` targets
- `.gitignore` updated with `backend/bin/`, `frontend/dist/`, `frontend/node_modules/`, `frontend/src/api/generated/`
- All acceptance criteria pass (health check, `pnpm dev`, `make test`, `pnpm generate`, `pnpm tsc --noEmit`)

## Notes

- Spec: `context/specs/01-scaffolding.md`
- Go module name: `github.com/polina2410/scout/backend`
- Backend is stdlib only at this stage — no external Go deps yet
- `make dev` starts Docker (MinIO) and prints instructions for the two terminal processes; does not background them
- `cmd/server/main.go` is a minimal health-check server only — real routes come in Step 5
- All npm deps must have been published at least one week ago before installing

## History

<!-- Keep this updated. Earliest to latest -->