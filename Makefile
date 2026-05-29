# shortr — build + dev orchestration

GO         ?= go
BUN        ?= bun
PKG        := github.com/erfianugrah/shortr
BIN        := bin/shortr
GIT_SHA    := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X main.version=$(GIT_SHA)
IMAGE      ?= ghcr.io/USER/shortr
IMAGE_TAG  ?= $(GIT_SHA)

# ---------- dev loop ----------

.PHONY: dev
dev: ## run Go server + Astro dev concurrently (Ctrl-C stops both)
	@./scripts/dev.sh

.PHONY: setup-web
setup-web: ## install web/ deps
	cd web && $(BUN) install

.PHONY: shadcn-init
shadcn-init: ## one-time: scaffold shadcn/ui in web/
	cd web && $(BUN) x shadcn@latest init

.PHONY: shadcn-default
shadcn-default: ## install the components used by the dashboard
	cd web && $(BUN) x shadcn@latest add button input label table form dialog dropdown-menu sonner

# ---------- code generation ----------

.PHONY: sqlc
sqlc: ## regenerate internal/storage/sqlitegen from queries
	sqlc generate

.PHONY: tidy
tidy:
	$(GO) mod tidy

# ---------- migrations ----------

.PHONY: migrate-up
migrate-up: ## apply all pending migrations to ./shortr.db
	$(GO) run . migrate up

.PHONY: migrate-down
migrate-down: ## roll back one migration
	$(GO) run . migrate down

.PHONY: migrate-status
migrate-status:
	$(GO) run . migrate status

# ---------- build ----------

.PHONY: web-build
web-build: ## build the dashboard
	cd web && $(BUN) run build

.PHONY: build
build: web-build ## build the binary (web first)
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .
	@echo "built $(BIN)"

.PHONY: build-go-only
build-go-only: ## go build without rebuilding web/
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

# ---------- quality ----------

.PHONY: test
test:
	$(GO) test -race -count=1 ./...

.PHONY: test-short
test-short:
	$(GO) test -short ./...

.PHONY: lint
lint: ## biome (web) + go vet
	cd web && $(BUN) x biome check
	$(GO) vet ./...

.PHONY: fmt
fmt:
	cd web && $(BUN) x biome check --write
	gofmt -w .

# ---------- docker / deploy ----------

.PHONY: image
image: ## build container image
	docker build -f deploy/Dockerfile -t $(IMAGE):$(IMAGE_TAG) -t $(IMAGE):latest .

.PHONY: image-push
image-push: image
	docker push $(IMAGE):$(IMAGE_TAG)
	docker push $(IMAGE):latest

.PHONY: deploy
deploy: ## deploy to fly using current image tag
	flyctl deploy --config deploy/fly.toml --image $(IMAGE):$(IMAGE_TAG)

.PHONY: deploy-remote
deploy-remote: ## let fly build remotely (no local docker required)
	flyctl deploy --config deploy/fly.toml --remote-only

.PHONY: backup
backup: ## take a manual snapshot of the prod volume (before risky changes)
	@vol=$$(flyctl volumes list --app shortr-erfi --json | jq -r '.[0].id'); \
	  echo "snapshotting $$vol"; \
	  flyctl volumes snapshots create $$vol --app shortr-erfi

.PHONY: snapshots
snapshots: ## list volume snapshots
	@vol=$$(flyctl volumes list --app shortr-erfi --json | jq -r '.[0].id'); \
	  flyctl volumes snapshots list $$vol --app shortr-erfi

# ---------- helpers ----------

.PHONY: clean
clean:
	rm -rf bin/ web/dist/ web/.astro/

.PHONY: help
help:
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[1m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
