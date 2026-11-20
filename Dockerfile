# ============================================
# Base stage - Common dependencies
# ============================================
FROM golang:1.26-alpine AS base

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .


# ============================================
# Development stage - With Air hot reload
# ============================================
FROM base AS development

RUN go install github.com/air-verse/air@v1.67.1
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
ENV PATH="/go/bin:${PATH}"

EXPOSE 3000

CMD ["air", "-c", ".air.toml"]


# ============================================
# Builder stage - Build binaries
# ============================================
FROM base AS builder

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o api \
    ./cmd/api

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o worker \
    ./cmd/worker

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o worker-scheduler \
    ./cmd/worker-scheduler

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o worker-calls \
    ./cmd/worker-calls

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o worker-notify \
    ./cmd/worker-notify

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o cli \
    ./cmd/cli


# ============================================
# Runtime base - shared by every production image
# ============================================
FROM alpine:latest AS runtime

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /home/appuser
USER appuser


# ============================================
# One image per service: each carries exactly one binary, so the
# worker-calls image (which runs untrusted HTTP) has no admin CLI and no API code.
# ============================================
FROM runtime AS api
COPY --from=builder --chown=appuser:appuser /app/api .
EXPOSE 3000
CMD ["./api"]

FROM runtime AS worker
COPY --from=builder --chown=appuser:appuser /app/worker .
CMD ["./worker"]

FROM runtime AS worker-scheduler
COPY --from=builder --chown=appuser:appuser /app/worker-scheduler .
CMD ["./worker-scheduler"]

FROM runtime AS worker-calls
COPY --from=builder --chown=appuser:appuser /app/worker-calls .
CMD ["./worker-calls"]

FROM runtime AS worker-notify
COPY --from=builder --chown=appuser:appuser /app/worker-notify .
CMD ["./worker-notify"]

FROM runtime AS cli
COPY --from=builder --chown=appuser:appuser /app/cli .
CMD ["./cli"]
