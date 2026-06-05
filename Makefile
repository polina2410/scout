.PHONY: dev build test seed server

dev:
	docker compose --env-file .env.local up -d
	@echo ""
	@echo "MinIO running at http://localhost:9000 (console: http://localhost:9001)"
	@echo "Start backend:  make server"
	@echo "Start frontend: cd frontend && pnpm dev"

# server loads .env.local into the environment, then runs the backend.
# Windows make runs recipes via cmd.exe (no grep/env/xargs), so it shells out to
# PowerShell; Unix uses the same inline env-loading pattern as the seed target.
ifeq ($(OS),Windows_NT)
server:
	powershell -NoProfile -Command "Get-Content .env.local | Where-Object { $$_ -match '^\s*[^#].+=' } | ForEach-Object { $$p = $$_ -split '=',2; [Environment]::SetEnvironmentVariable($$p[0].Trim(), $$p[1].Trim()) }; Set-Location backend; go run ./cmd/server"
else
server:
	cd backend && env $$(grep -v '^#' ../.env.local | grep -v '^$$' | xargs) go run ./cmd/server
endif

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && pnpm build

test:
	cd backend && go test ./...
	cd frontend && pnpm test

# seed loads .env.local and runs the seeder. Same OS split as the server target:
# PowerShell on Windows (no grep/env/xargs under cmd.exe), inline env on Unix.
ifeq ($(OS),Windows_NT)
seed:
	powershell -NoProfile -Command "Get-Content .env.local | Where-Object { $$_ -match '^\s*[^#].+=' } | ForEach-Object { $$p = $$_ -split '=',2; [Environment]::SetEnvironmentVariable($$p[0].Trim(), $$p[1].Trim()) }; Set-Location backend; go run ./cmd/seed"
else
seed:
	cd backend && env $$(grep -v '^#' ../.env.local | grep -v '^$$' | xargs) go run ./cmd/seed
endif
