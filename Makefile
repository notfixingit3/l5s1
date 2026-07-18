.PHONY: up down rebuild logs test run data-backup data-reset dev help security gosec govulncheck lint-fe vet

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

VERSION := $(shell tr -d '[:space:]' < VERSION 2>/dev/null || echo 0.0.0-dev)

up: ## Build & start Docker stack (passkeys persist in ./data)
	mkdir -p data
	docker compose build --build-arg VERSION=$(VERSION) --build-arg COMMIT=local --build-arg BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ)
	docker compose up -d
	@echo "Open http://localhost:8080  (use localhost, not 127.0.0.1) · v$(VERSION)"

pull-ghcr: ## Run published image (IMAGE_TAG=v0.0.1-beta.19)
	mkdir -p data
	docker compose -f docker-compose.ghcr.yml pull
	docker compose -f docker-compose.ghcr.yml up -d
	@echo "Open http://localhost:8080 · image tag $${IMAGE_TAG:-v0.0.1-beta.19}"

dev: ## Start with host frontend bind-mount for UI iteration
	mkdir -p data
	docker compose --profile dev up --build -d app-dev
	@echo "Open http://localhost:8080  (frontend live from ./frontend)"

down: ## Stop containers (keeps ./data passkeys)
	docker compose --profile dev down

rebuild: ## Rebuild image without wiping data
	mkdir -p data
	docker compose up --build -d

logs: ## Tail app logs
	docker compose logs -f app

test: ## Run Go tests on host
	cd backend && go test ./...

run: ## Run API on host (uses ./data for SQLite — same passkeys as Docker if shared)
	mkdir -p data
	cd backend && DATA_DIR=../data DATABASE_DSN='file:../data/l5s1.db?cache=shared&mode=rwc' \
		FRONTEND_DIR=../frontend WEBAUTHN_RP_ID=localhost \
		WEBAUTHN_ORIGINS=http://localhost:8080 go run ./cmd/server

data-backup: ## Snapshot SQLite (passkeys + logs) to data/backups/
	mkdir -p data/backups
	@ts=$$(date +%Y%m%d-%H%M%S); \
	cp -a data/l5s1.db "data/backups/l5s1-$$ts.db" 2>/dev/null || true; \
	echo "Backup: data/backups/l5s1-$$ts.db (if db existed)"

data-reset: ## DANGER: delete local DB (all passkeys/users/logs)
	@echo "This deletes ./data/l5s1.db and all registered passkeys."
	@read -p "Type YES to confirm: " ans; [ "$$ans" = "YES" ] || exit 1
	rm -f data/l5s1.db data/l5s1.db-*
	@echo "Data wiped. Next start will re-seed config/admin shell."

# —— Security ——

security: vet gosec govulncheck lint-fe ## Full security suite (Go + frontend)
	@echo "All security checks finished."

vet: ## go vet
	cd backend && go vet ./...

gosec: ## gosec static analysis
	cd backend && gosec -fmt text ./...

govulncheck: ## govulncheck dependency CVEs
	cd backend && govulncheck ./...

lint-fe: ## ESLint + security plugin on frontend JS
	cd frontend && npm install --no-save eslint eslint-plugin-security >/dev/null 2>&1 && npx eslint js/
