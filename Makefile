.PHONY: run docker-build security-scan check-deps

run:
	go run ./app/main.go

docker-build:
	docker build -f pkg/Dockerfile -t dashie .

check-deps:
	@command -v trivy >/dev/null 2>&1 || { echo "trivy is not installed. Install it: https://aquasecurity.github.io/trivy"; exit 1; }
	@echo "All dependencies found."

security-scan: check-deps
	trivy image dashie