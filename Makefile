.PHONY: run run-live docker-build security-scan check-deps

run:
	go run ./app/main.go

run-live: check-deps
	air

docker-build:
	docker build -f pkg/Dockerfile -t dash .

check-deps:
	@command -v trivy >/dev/null 2>&1 || { echo "trivy is not installed. Install it: https://aquasecurity.github.io/trivy"; exit 1; }
	@command -v air >/dev/null 2>&1 || { echo "air is not installed. Install it: go install github.com/air-verse/air@latest"; exit 1; }
	@echo "All dependencies found."

security-scan: check-deps
	trivy image dash