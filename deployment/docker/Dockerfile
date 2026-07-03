# syntax=docker/dockerfile:1
# Dockerfile multi-stage: builder golang:1.26.4-alpine + runtime gcr.io/distroless/static-debian12:nonroot
# ADR-011: imagem final ≤ 30 MB; roda como UID 65532 (nonroot)
#
# Modos de build:
#   BUILD_MODE=build    (padrao) compila o binario dentro do container.
#   BUILD_MODE=artifact usa bin/mecontrola pre-compilado (CI) para evitar recompilacao.
#   Em CI, use --target runtime com BUILD_MODE=artifact para pular o stage builder.

ARG BUILD_MODE=build
ARG VERSION=dev

# ---- builder ----
FROM golang:1.26.4-alpine@sha256:0648ddfa35769070197ba1cdf22a16dc452caf9315e66b91791308a543baf229 AS builder

ARG VERSION
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

# Cache de modulos separado do codigo-fonte (layer cache eficiente)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/mecontrola \
      ./cmd

# ---- artifact provider ----
# Usa binario pre-compilado fornecido pelo CI (bin/mecontrola).
FROM alpine:3.20@sha256:c64c687cbea9300178b30c95835354e34c4e4febc4badfe27102879de0483b5e AS artifact-provider
WORKDIR /out
COPY bin/mecontrola /out/mecontrola

# ---- build provider ----
# Apenas reutiliza o binario compilado no stage builder.
FROM builder AS build-provider

# ---- selected provider ----
FROM ${BUILD_MODE}-provider AS selected

# ---- tools ----
FROM alpine:3.20@sha256:c64c687cbea9300178b30c95835354e34c4e4febc4badfe27102879de0483b5e AS tools
RUN apk add --no-cache busybox-static

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot@sha256:b669b9df05a88a085fefed6520c6d2268aabacf3008b149ddf877e752ae89400 AS runtime

# UID 65532 = nonroot (vem embutido na imagem distroless:nonroot)
USER nonroot

WORKDIR /app

COPY --from=tools /bin/busybox.static /usr/bin/wget
COPY --from=tools /bin/busybox.static /usr/bin/pgrep

COPY --from=selected /out/mecontrola /app/mecontrola

# OpenAPI specs usados pelo docs router em ambiente local (ENVIRONMENT=local).
# O codigo faz fallback para /app/internal/<modulo>/openapi.yaml quando
# runtime.Caller nao resolve por causa de -trimpath.
COPY internal/identity/openapi.yaml /app/internal/identity/openapi.yaml
COPY internal/billing/openapi.yaml /app/internal/billing/openapi.yaml
COPY internal/categories/openapi.yaml /app/internal/categories/openapi.yaml
COPY internal/onboarding/openapi.yaml /app/internal/onboarding/openapi.yaml
COPY internal/card/openapi.yaml /app/internal/card/openapi.yaml
COPY internal/budgets/openapi.yaml /app/internal/budgets/openapi.yaml
COPY internal/transactions/openapi.yaml /app/internal/transactions/openapi.yaml

ENTRYPOINT ["/app/mecontrola"]
CMD ["server"]
