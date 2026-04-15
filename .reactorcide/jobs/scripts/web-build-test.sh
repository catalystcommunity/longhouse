#!/usr/bin/env bash
set -euo pipefail

echo "================================================"
echo "Longhouse Web UI Docker Build Test"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"

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

    docker build -t longhouse-web:test -f Dockerfile.web .
    echo "=== Web UI Docker image builds successfully ==="
else
    echo "ERROR: Docker not available"
    exit 1
fi
