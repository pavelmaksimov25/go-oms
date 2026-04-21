#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing Go tools..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
go install sigs.k8s.io/kind@latest

echo "==> Installing buf..."
if ! command -v buf &>/dev/null; then
    echo "    Install buf manually: https://buf.build/docs/installation"
    echo "    macOS: brew install bufbuild/buf/buf"
else
    echo "    buf already installed: $(buf --version)"
fi

echo "==> Done. You may also need:"
echo "    - Docker Desktop (https://www.docker.com/products/docker-desktop)"
echo "    - kubectl (https://kubernetes.io/docs/tasks/tools/)"
