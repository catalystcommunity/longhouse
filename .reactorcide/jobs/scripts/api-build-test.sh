#!/usr/bin/env bash
set -euo pipefail

echo "================================================"
echo "Longhouse API Docker Build Test"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"

export HOME="${HOME:-/tmp/home}"
LOCAL_BIN="$HOME/.local/bin"
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

if [[ -n "${DOCKER_HOST:-}" ]]; then
    if ! command -v docker &> /dev/null; then
        echo "Installing docker CLI..."
        DOCKER_VERSION=27.5.1
        curl -fsSL "https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz" -o /tmp/docker.tgz
        tar -xzf /tmp/docker.tgz --strip-components=1 -C /usr/local/bin docker/docker
        rm /tmp/docker.tgz
    fi

    echo "Waiting for Docker daemon..."
    for i in $(seq 1 30); do
        if docker info >/dev/null 2>&1; then
            echo "Docker daemon is ready"
            break
        fi
        if [[ $i -eq 30 ]]; then
            echo "ERROR: Docker daemon not ready after 30 seconds"
            exit 1
        fi
        sleep 1
    done

    docker build -t longhouse-api:test -f Dockerfile.api .
    echo "=== API Docker image builds successfully ==="
else
    if ! command -v buildctl &> /dev/null; then
        echo "Installing buildctl..."
        BUILDKIT_VERSION=0.17.3
        curl -fsSL "https://github.com/moby/buildkit/releases/download/v${BUILDKIT_VERSION}/buildkit-v${BUILDKIT_VERSION}.linux-amd64.tar.gz" -o /tmp/buildkit.tar.gz
        tar -xzf /tmp/buildkit.tar.gz --strip-components=1 -C "$LOCAL_BIN" bin/buildctl
        rm /tmp/buildkit.tar.gz
    fi

    echo "Waiting for builder sidecar..."
    for i in $(seq 1 30); do
        if buildctl debug info >/dev/null 2>&1; then
            echo "builder sidecar is ready"
            break
        fi
        if [[ $i -eq 30 ]]; then
            echo "ERROR: builder sidecar not ready after 30 seconds"
            exit 1
        fi
        sleep 1
    done

    buildctl build \
        --frontend dockerfile.v0 \
        --local context=. \
        --local dockerfile=. \
        --opt filename=Dockerfile.api \
        --output type=oci,dest=/tmp/longhouse-api-image.tar

    rm -f /tmp/longhouse-api-image.tar
    echo "=== API Docker image builds successfully ==="
fi
