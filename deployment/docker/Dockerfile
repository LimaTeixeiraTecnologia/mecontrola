# syntax=docker/dockerfile:1
# Dockerfile multi-stage: builder golang:1.26.4-alpine + runtime gcr.io/distroless/static-debian12:nonroot
# ADR-011: imagem final ≤ 30 MB; roda como UID 65532 (nonroot)

# ---- builder ----
FROM golang:1.26.4-alpine AS builder

ARG VERSION=dev
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

# Cache de módulos separado do código-fonte (layer cache eficiente)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/mecontrola \
      ./cmd

# ---- tools ----
FROM alpine:3.20 AS tools
RUN apk add --no-cache busybox-static

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot

# UID 65532 = nonroot (vem embutido na imagem distroless:nonroot)
USER nonroot

WORKDIR /app

COPY --from=builder /out/mecontrola /app/mecontrola
COPY --from=tools /bin/busybox.static /usr/bin/wget
COPY --from=tools /bin/busybox.static /usr/bin/pgrep

# OpenAPI specs usados pelo docs router em ambiente local (ENVIRONMENT=local).
# O codigo faz fallback para /app/internal/<modulo>/openapi.yaml quando
# runtime.Caller nao resolve por causa de -trimpath.
COPY --from=builder /build/internal/identity/openapi.yaml /app/internal/identity/openapi.yaml
COPY --from=builder /build/internal/billing/openapi.yaml /app/internal/billing/openapi.yaml
COPY --from=builder /build/internal/categories/openapi.yaml /app/internal/categories/openapi.yaml
COPY --from=builder /build/internal/onboarding/openapi.yaml /app/internal/onboarding/openapi.yaml
COPY --from=builder /build/internal/card/openapi.yaml /app/internal/card/openapi.yaml
COPY --from=builder /build/internal/budgets/openapi.yaml /app/internal/budgets/openapi.yaml
COPY --from=builder /build/internal/transactions/openapi.yaml /app/internal/transactions/openapi.yaml

ENTRYPOINT ["/app/mecontrola"]
CMD ["server"]
