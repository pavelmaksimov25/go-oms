FROM golang:1.25-alpine AS builder

ARG SERVICE_NAME

WORKDIR /app

# Copy go.work and all module files for dependency resolution
COPY go.work .
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/${SERVICE_NAME}/go.mod services/${SERVICE_NAME}/go.sum ./services/${SERVICE_NAME}/

# We need all go.mod files referenced in go.work for the workspace to resolve.
# Copy them as stubs if they don't exist for the target service.
COPY services/ ./services/

# Download dependencies
RUN cd services/${SERVICE_NAME} && go mod download

# Copy source code
COPY pkg/ ./pkg/
COPY services/${SERVICE_NAME}/ ./services/${SERVICE_NAME}/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/service ./services/${SERVICE_NAME}

# ── Runtime ──────────────────────────────────────────────────────────────────

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/service /bin/service

EXPOSE 50051

ENTRYPOINT ["/bin/service"]
