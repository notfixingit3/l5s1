# syntax=docker/dockerfile:1
# L5S1 — multi-stage image (pure Go SQLite, CGO-free)
# Tests run in CI (make test), not here — keeps multi-arch image builds fast.

ARG VERSION=0.0.1-beta.18
ARG COMMIT=dev
ARG BUILD_TIME=unknown

FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build
ARG VERSION
ARG COMMIT
ARG BUILD_TIME
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ ./
COPY frontend/ /frontend/

# Stamp version into SPA for footer display
RUN printf '/** Build-stamped version */\nexport const APP_VERSION = "%s";\nexport const APP_COMMIT = "%s";\n' \
      "$VERSION" "$COMMIT" > /frontend/js/version.js

ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go version \
 && GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
      -ldflags="-s -w \
        -X github.com/l5s1/health-registry/internal/version.Version=${VERSION} \
        -X github.com/l5s1/health-registry/internal/version.Commit=${COMMIT} \
        -X github.com/l5s1/health-registry/internal/version.BuildTime=${BUILD_TIME}" \
      -o /out/l5s1 ./cmd/server

FROM debian:bookworm-slim
ARG VERSION
ARG COMMIT
ARG BUILD_TIME

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /data /app

WORKDIR /app
COPY --from=build /out/l5s1 /app/l5s1
COPY --from=build /frontend /app/frontend

LABEL org.opencontainers.image.title="L5S1 Health Registry" \
      org.opencontainers.image.description="Passwordless multi-condition health tracking PWA" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/notfixingit3/l5s1"

ENV DATA_DIR=/data \
    DATABASE_DSN="file:/data/l5s1.db?cache=shared&mode=rwc" \
    FRONTEND_DIR=/app/frontend \
    PORT=8080 \
    WEBAUTHN_RP_ID=localhost \
    WEBAUTHN_ORIGINS=http://localhost:8080 \
    WEBAUTHN_RP_DISPLAY_NAME="L5S1 Health Registry" \
    SEED_ADMIN_USERNAME=admin \
    GIN_MODE=release \
    SECURE_COOKIE=false \
    L5S1_VERSION=${VERSION}

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/api/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/l5s1"]
