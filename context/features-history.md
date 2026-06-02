# Features History

<!-- Completed features are appended here by /feature complete -->

## Project Structure & Tooling

**Branch:** project-structure-and-tooling
**Completed:** 2026-06-02

### Goals

- Go module initialized (`github.com/polina2410/scout/backend`) with 6 internal package stubs
- Vite + React 19 + TypeScript frontend scaffolded with all required dependencies
- Feature-based folder structure: `features/gallery/`, `features/filters/`, `features/map/`, `store/`
- `vite.config.ts`: dev proxy, jsdom test env, test setup file
- `tsconfig.app.json`: strict, noImplicitAny, noUncheckedIndexedAccess
- `openapi-typescript` codegen wired as `pnpm generate`
- Makefile with `dev`, `build`, `test`, `seed` targets
- `.gitignore` updated for `backend/bin/`, `frontend/dist/`, `frontend/node_modules/`, `frontend/src/api/generated/`

### Summary

Empty but runnable skeleton. `make dev` starts Docker (MinIO) and prints instructions for running backend and frontend in separate terminals. `make test` exits 0 with zero tests. `pnpm generate` produces typed API schema from `openapi.yaml`. All acceptance criteria pass.
