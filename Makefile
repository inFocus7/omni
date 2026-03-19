CLUSTER_NAME ?=omni-dev
OMNI_NAMESPACE ?=omni-system

.PHONY: run run-live docker-build docker-run kind-setup kind-forward security-scan check-deps minify-js helm-lint kind-down db-shell db-dump test test-e2e

# ── Local development ─────────────────────────────────────────────────────────

minify-js:
	esbuild ui/static/app.js --minify --outfile=ui/static/app.min.js

run: minify-js
	go run ./app

run-live: check-deps minify-js
	air

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build: minify-js
	docker build -f pkg/Dockerfile -t omni:latest .

# Run the container locally. Pass GITHUB_TOKEN via env.
# Usage: make docker-run GITHUB_TOKEN=ghp_...
docker-run: docker-build
	docker run --rm -p 8080:8080 \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-v omni-data:/data \
		-e OMNI_DATA_DIR=/data \
		omni:latest

# ── Kubernetes (kind) ─────────────────────────────────────────────────────────

# Full workflow: create cluster → build image → load into kind → helm install
# Usage: make kind-setup GITHUB_TOKEN=ghp_...
kind-setup:
	GITHUB_TOKEN=$(GITHUB_TOKEN) ./scripts/kind-setup.sh

# Forward the service port after kind-setup is done
kind-forward:
	kubectl port-forward -n $(OMNI_NAMESPACE) svc/omni 8080:8080

# Delete the kind cluster
kind-down:
	kind delete cluster --name $(CLUSTER_NAME)

# ── Database ──────────────────────────────────────────────────────────────────

LOCAL_DB ?= dashie.db
POD := $(shell kubectl get pod -n $(OMNI_NAMESPACE) -l app.kubernetes.io/name=omni -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
POD_DB_PATH ?= /data/dashie.db

# Open an interactive sqlite3 shell against the local DB (requires sqlite3)
db-shell:
	sqlite3 $(LOCAL_DB)

# Dump all rows from all tables in the local DB
# Example one-liner: sqlite3 dashie.db ".headers on" ".mode column" "SELECT * FROM animations;"
db-dump:
	@sqlite3 $(LOCAL_DB) \
		".headers on" \
		".mode column" \
		"SELECT '=== animations ===' AS '';" \
		"SELECT name, source FROM animations;" \
		"SELECT '=== variants ===' AS '';" \
		"SELECT name, size, cols, rows, fps FROM variants;" \
		"SELECT '=== animation_frames ===' AS '';" \
		"SELECT name, size, length(frames) AS frames_bytes FROM animation_frames;"

# ── Testing ───────────────────────────────────────────────────────────────────

test:
	go test ./...

test-e2e:
	go test -v -timeout 60s ./pkg/api/ -run 'TestImport|TestExport'

# ── Utilities ─────────────────────────────────────────────────────────────────

helm-lint:
	helm lint ./helm

check-deps:
	@command -v trivy >/dev/null 2>&1 || { echo "trivy is not installed. Install it: https://aquasecurity.github.io/trivy"; exit 1; }
	@command -v air >/dev/null 2>&1 || { echo "air is not installed. Install it: go install github.com/air-verse/air@latest"; exit 1; }
	@echo "All dependencies found."

security-scan: docker-build check-deps
	trivy image omni:latest
