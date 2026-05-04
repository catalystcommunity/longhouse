#!/usr/bin/env bash
set -euo pipefail

echo "================================================"
echo "Longhouse Deploy"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"

if [[ -z "${K8S_NAMESPACE:-}" ]]; then
    echo "ERROR: K8S_NAMESPACE must be set via overlay"
    exit 1
fi
if [[ -z "${HELM_RELEASE:-}" ]]; then
    echo "ERROR: HELM_RELEASE must be set via overlay"
    exit 1
fi
if [[ -z "${HELM_VALUES_FILE:-}" ]]; then
    echo "ERROR: HELM_VALUES_FILE must be set via overlay"
    exit 1
fi

# Default IMAGE_TAG to whatever VERSION.txt says (the canonical "what's
# the released version right now"), so a deploy fired on a release-bump
# commit lines up with the build-and-deploy job that just pushed the
# image. Falls back to "latest" if VERSION.txt is missing or empty.
if [[ -z "${IMAGE_TAG:-}" ]]; then
    if [[ -s version/VERSION.txt ]]; then
        IMAGE_TAG="$(tr -d '[:space:]' < version/VERSION.txt)"
    else
        IMAGE_TAG="latest"
    fi
fi

echo "Namespace:  ${K8S_NAMESPACE}"
echo "Release:    ${HELM_RELEASE}"
echo "Values:     ${HELM_VALUES_FILE}"
echo "Image tag:  ${IMAGE_TAG}"

# ================================================
# Setup tools
# ================================================
export HOME="${HOME:-/root}"
LOCAL_BIN="$HOME/.local/bin"
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

if ! command -v helm &> /dev/null; then
    echo "Installing helm..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | USE_SUDO=false HELM_INSTALL_DIR="$LOCAL_BIN" bash
fi

if ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
    KUBECTL_VERSION=$(curl -fsSL https://dl.k8s.io/release/stable.txt)
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" -o "$LOCAL_BIN/kubectl"
    chmod +x "$LOCAL_BIN/kubectl"
fi

# ================================================
# Configure kubectl
# ================================================
mkdir -p ~/.kube
echo "${KUBECONFIG_CONTENT}" > ~/.kube/config
chmod 600 ~/.kube/config

# ================================================
# Create namespace and registry secret
# ================================================
kubectl create namespace "${K8S_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

if [[ -n "${REGISTRY_USER:-}" ]] && [[ -n "${REGISTRY_PASSWORD:-}" ]]; then
    kubectl create secret docker-registry regcred \
        --namespace "${K8S_NAMESPACE}" \
        --save-config \
        --dry-run=client \
        --docker-server="containers.catalystsquad.com" \
        --docker-username="${REGISTRY_USER}" \
        --docker-password="${REGISTRY_PASSWORD}" \
        -o yaml | kubectl apply -f -
fi

# ================================================
# Deploy with Helm
# ================================================
echo ""
echo "================================================"
echo "Deploying with Helm"
echo "================================================"

# Write runtime overrides to a temp values file. The file may hold
# secret-sourced values (LINKKEYS_PKI_API_KEY, etc.) that we don't want
# lingering, so chmod it tight and `shred -u` on EXIT.
RUNTIME_VALUES="/tmp/runtime-values.yaml"
( umask 077 && : > "${RUNTIME_VALUES}" )
trap 'shred -u "${RUNTIME_VALUES}" 2>/dev/null || rm -f "${RUNTIME_VALUES}"' EXIT

{
    cat <<VALS
image:
  api:
    tag: "${IMAGE_TAG}"
  web:
    tag: "${IMAGE_TAG}"
VALS
    if [[ -n "${LINKKEYS_PKI_API_KEY:-}" ]]; then
        # Quoted YAML scalar — base64url + literal '.' from the linkkeys
        # api-key format are all safe inside double-quoted YAML strings.
        cat <<VALS
linkkeysRp:
  apiKey: "${LINKKEYS_PKI_API_KEY}"
VALS
    fi
} > "${RUNTIME_VALUES}"

helm upgrade \
    --install \
    --create-namespace \
    --namespace "${K8S_NAMESPACE}" \
    "${HELM_RELEASE}" \
    ./helm_chart \
    -f "${HELM_VALUES_FILE}" \
    -f "${RUNTIME_VALUES}"

echo ""
echo "================================================"
echo "Deployment complete!"
echo "Namespace: ${K8S_NAMESPACE}"
echo "Release:   ${HELM_RELEASE}"
echo "================================================"
