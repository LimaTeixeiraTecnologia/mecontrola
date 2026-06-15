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

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot

# UID 65532 = nonroot (vem embutido na imagem distroless:nonroot)
USER nonroot

WORKDIR /app

COPY --from=builder /out/mecontrola /app/mecontrola
COPY --from=tools /usr/bin/wget /usr/bin/wget
COPY --from=tools /usr/bin/pgrep /usr/bin/pgrep

ENTRYPOINT ["/app/mecontrola"]
CMD ["server"]
