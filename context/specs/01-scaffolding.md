# Spec 01 — Project Structure & Tooling

**Plan ref:** Phase 1, Step 1  
**Goal:** Empty but runnable repo skeleton. After this step `make dev` starts both services, `make test` runs (zero tests, exit 0), and the directory layout matches CLAUDE.md.

---

## Directory structure to create

```
scout-takehome/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go        # "hello" HTTP server on $PORT
│   ├── internal/
│   │   ├── config/            # empty package placeholder
│   │   ├── db/                # empty package placeholder
│   │   ├── handler/           # empty package placeholder
│   │   ├── middleware/        # empty package placeholder
│   │   ├── minio/             # empty package placeholder
│   │   └── thumb/             # empty package placeholder
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── features/
│   │   │   ├── gallery/       # empty, index.ts placeholder
│   │   │   ├── filters/       # empty, index.ts placeholder
│   │   │   └── map/           # empty, index.ts placeholder
│   │   ├── store/             # empty, index.ts placeholder
│   │   └── api/
│   │       └── generated/     # openapi-typescript output goes here (gitignored)
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
└── Makefile
```

---

## Backend

### Go module

```
module github.com/polina2410/scout/backend

go 1.23
```

Run: `cd backend && go mod init github.com/polina2410/scout/backend`

### `cmd/server/main.go`

Minimal HTTP server that:
- Reads `PORT` from env (default `8080`)
- Responds `200 OK` with `{"status":"ok"}` on `GET /health`
- Logs `listening on :PORT` to stdout and exits

No external dependencies yet — stdlib only.

---

## Frontend

### Init

```sh
cd frontend
pnpm create vite . --template react-ts
```

### Dependencies to install

```sh
# Runtime
pnpm add @reduxjs/toolkit react-redux react-konva konva

# Dev
pnpm add -D openapi-typescript vitest @vitest/ui jsdom @testing-library/react @testing-library/jest-dom
```

Check publish dates on npm before installing — all must be at least one week old.

### `vite.config.ts`

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        rewrite: path => path.replace(/^\/api/, ''),
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
  },
})
```

### `tsconfig.json` strict flags (required)

Ensure these are set:
```json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "noUncheckedIndexedAccess": true
  }
}
```

### openapi-typescript codegen script

Add to `package.json` scripts:
```json
"generate": "openapi-typescript ../openapi.yaml -o src/api/generated/schema.ts"
```

Run once after setup: `pnpm generate`. Output file is gitignored (generated artifact).

Add to `.gitignore`:
```
frontend/src/api/generated/
```

---

## Makefile

```makefile
.PHONY: dev build test seed

dev:
	docker compose --env-file .env.local up -d
	@echo "MinIO running. Start backend: cd backend && go run ./cmd/server"
	@echo "Start frontend: cd frontend && pnpm dev"

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && pnpm build

test:
	cd backend && go test ./...
	cd frontend && pnpm vitest run

seed:
	cd backend && go run ./cmd/seed
```

Note: `make dev` starts Docker but prints instructions for the two processes rather than running them in background — avoids requiring `concurrently` or similar tools as a setup dependency. Developers run backend and frontend in separate terminals.

---

## Gitignore additions

Add to `.gitignore`:
```
# Go
backend/bin/

# Generated
frontend/src/api/generated/

# Frontend build
frontend/dist/
frontend/node_modules/
```

---

## Acceptance criteria

- [ ] `cd backend && go run ./cmd/server` starts and `curl localhost:8080/health` returns `{"status":"ok"}`
- [ ] `cd frontend && pnpm dev` starts without errors on port 5173
- [ ] `make test` exits 0 (zero tests is fine at this stage)
- [ ] `pnpm generate` produces `src/api/generated/schema.ts` from `openapi.yaml`
- [ ] TypeScript strict mode: `pnpm tsc --noEmit` exits 0
- [ ] No `any` in generated or hand-written code
- [ ] Directory structure matches the layout above