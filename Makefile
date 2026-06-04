.PHONY: dev build test seed

dev:
	docker compose --env-file .env.local up -d
	@echo ""
	@echo "MinIO running at http://localhost:9000 (console: http://localhost:9001)"
	@echo "Start backend:  cd backend && go run ./cmd/server"
	@echo "Start frontend: cd frontend && pnpm dev"

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && pnpm build

test:
	cd backend && go test ./...
	cd frontend && pnpm test

seed:
	cd backend && env $$(grep -v '^#' ../.env.local | grep -v '^$$' | xargs) go run ./cmd/seed
