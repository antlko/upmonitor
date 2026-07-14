# syntax=docker/dockerfile:1

# --- Stage 1: build the Vue single-page app ---
FROM node:22-alpine AS web
WORKDIR /web
COPY web-ui/package.json web-ui/package-lock.json ./
RUN npm ci
COPY web-ui/ ./
RUN npm run build

# --- Stage 2: build the static Go binary (embeds the SPA) ---
# Runs natively on the build platform and cross-compiles to the target arch,
# which is fast and safe because the SQLite driver is pure Go (CGO disabled).
FROM --platform=$BUILDPLATFORM golang:1.25 AS build
# TARGETOS/TARGETARCH are injected by BuildKit from the target platform
# (the host's arch for a plain `docker build`, or each --platform under buildx).
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Embed the freshly built SPA into the binary.
COPY --from=web /web/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /upmonitor ./cmd/upmonitor

# --- Stage 3: minimal runtime image ---
FROM gcr.io/distroless/static:latest
LABEL org.opencontainers.image.title="upmonitor" \
      org.opencontainers.image.description="Simple self-hosted service monitoring" \
      org.opencontainers.image.source="https://github.com/anatolkzh/upmonitor"
COPY --from=build /upmonitor /upmonitor
ENV UPMONITOR_CONFIG_DIR=/config \
    UPMONITOR_ADDR=:8080
EXPOSE 8080
VOLUME ["/config"]
ENTRYPOINT ["/upmonitor"]
