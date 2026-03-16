#!/usr/bin/env bash
# kind-setup.sh — Create a kind cluster, build the omni image, and install via Helm.
#
# Usage:
#   ./scripts/kind-setup.sh                     # defaults
#   GITHUB_TOKEN=ghp_... ./scripts/kind-setup.sh
#   CLUSTER_NAME=my-cluster ./scripts/kind-setup.sh
#   OMNI_IMAGE_TAG=v0.2.0 ./scripts/kind-setup.sh
#
# Environment variables:
#   CLUSTER_NAME      kind cluster name            (default: omni-dev)
#   OMNI_NAMESPACE    Kubernetes namespace          (default: omni-system)
#   OMNI_IMAGE_TAG    Docker image tag             (default: dev)
#   GITHUB_TOKEN      GitHub personal access token (optional)
#   SKIP_BUILD        Set to "true" to skip docker build (default: unset)
#   SKIP_CLUSTER      Set to "true" to skip cluster creation (default: unset)

set -euo pipefail

# ── Config ───────────────────────────────────────────────────────────────────
CLUSTER_NAME="${CLUSTER_NAME:-omni-dev}"
OMNI_NAMESPACE="${OMNI_NAMESPACE:-omni-system}"
OMNI_IMAGE="${OMNI_IMAGE:-omni}"
OMNI_IMAGE_TAG="${OMNI_IMAGE_TAG:-dev}"
FULL_IMAGE="${OMNI_IMAGE}:${OMNI_IMAGE_TAG}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── Colour helpers ────────────────────────────────────────────────────────────
bold=$'\e[1m'; reset=$'\e[0m'; green=$'\e[32m'; yellow=$'\e[33m'; red=$'\e[31m'
info()  { echo "${bold}${green}▶ $*${reset}"; }
warn()  { echo "${bold}${yellow}⚠ $*${reset}"; }
fatal() { echo "${bold}${red}✗ $*${reset}" >&2; exit 1; }

# ── Dependency checks ─────────────────────────────────────────────────────────
check_dep() {
  command -v "$1" &>/dev/null || fatal "$1 is required but not installed. $2"
}
check_dep kind    "Install: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
check_dep kubectl "Install: https://kubernetes.io/docs/tasks/tools/"
check_dep helm    "Install: https://helm.sh/docs/intro/install/"
check_dep docker  "Install: https://docs.docker.com/get-docker/"

# ── Step 1: kind cluster ──────────────────────────────────────────────────────
if [[ "${SKIP_CLUSTER:-}" == "true" ]]; then
  warn "Skipping cluster creation (SKIP_CLUSTER=true)"
else
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    warn "Cluster '${CLUSTER_NAME}' already exists — skipping creation"
  else
    info "Creating kind cluster '${CLUSTER_NAME}'"
    kind create cluster --name "${CLUSTER_NAME}"
  fi
fi

# Set kubectl context to the kind cluster
kubectl config use-context "kind-${CLUSTER_NAME}"

# ── Step 2: Build Docker image ────────────────────────────────────────────────
if [[ "${SKIP_BUILD:-}" == "true" ]]; then
  warn "Skipping image build (SKIP_BUILD=true)"
else
  info "Building Docker image '${FULL_IMAGE}'"
  docker build \
    -f "${REPO_ROOT}/pkg/Dockerfile" \
    -t "${FULL_IMAGE}" \
    "${REPO_ROOT}"
fi

# ── Step 3: Load image into kind ──────────────────────────────────────────────
info "Loading '${FULL_IMAGE}' into kind cluster '${CLUSTER_NAME}'"
kind load docker-image "${FULL_IMAGE}" --name "${CLUSTER_NAME}"

# ── Step 4: Create namespace ──────────────────────────────────────────────────
info "Ensuring namespace '${OMNI_NAMESPACE}'"
kubectl create namespace "${OMNI_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# ── Step 5: Install / upgrade Helm chart ─────────────────────────────────────
info "Installing Helm chart into namespace '${OMNI_NAMESPACE}'"

HELM_ARGS=(
  upgrade --install omni
  "${REPO_ROOT}/helm"
  --namespace "${OMNI_NAMESPACE}"
  --set "image.repository=${OMNI_IMAGE}"
  --set "image.tag=${OMNI_IMAGE_TAG}"
  --set "image.pullPolicy=Never"        # image is loaded locally, never pull
  --wait
  --timeout 120s
)

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  HELM_ARGS+=(--set "github.token=${GITHUB_TOKEN}")
else
  warn "GITHUB_TOKEN not set — GitHub widgets will not function"
fi

helm "${HELM_ARGS[@]}"

# ── Step 6: Force pod restart so the new image is actually used ───────────────
# helm upgrade with a static image tag + pullPolicy=Never does not restart the
# pod unless the Deployment spec changes. rollout restart forces a new pod with
# the freshly-loaded image.
info "Restarting deployment to pick up new image"
kubectl rollout restart deployment/omni --namespace "${OMNI_NAMESPACE}"
kubectl rollout status  deployment/omni --namespace "${OMNI_NAMESPACE}" --timeout 60s

# ── Step 7: Summary ───────────────────────────────────────────────────────────
info "Done! omni is running in namespace '${OMNI_NAMESPACE}'"
echo
echo "  Run 'make kind-forward' to forward the port, then open http://localhost:8080"
