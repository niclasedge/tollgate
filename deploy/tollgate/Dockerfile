# tollgate image: frontend and binary in one artifact (ADR-0002, ADR-0012).
# Build context: repository root (see docker-compose.yml and .dockerignore).
# Base images carry tag and multi-arch index digest (#59); update: docs/DEPLOY.md.

FROM node:22-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d AS web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
WORKDIR /src/web
RUN corepack enable
# pnpm runs inside web/ so corepack picks the version pinned in package.json.
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.26-bookworm@sha256:632dff7351799a5a9954a48dc06670ade72ee44e8c351cdc0e5a57af2fea21af AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Root package (fixtures.go) embeds testdata/fixtures; config/schema embeds
# the YAML schema. deploy/buildcontext_test.go checks these inputs (#107).
COPY fixtures.go ./
COPY testdata/fixtures ./testdata/fixtures
COPY config ./config
COPY cmd ./cmd
COPY internal ./internal
COPY web/*.go web/placeholder.html ./web/
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/tollgate ./cmd/tollgate

FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8
# iproute2: the entrypoint reads the Tailnet IP; util-linux (base) brings setpriv.
RUN apt-get update \
    && apt-get install -y --no-install-recommends iproute2 ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10002 --home-dir /nonexistent --shell /usr/sbin/nologin tollgate \
    && mkdir -p /data /run/tollgate /alerts \
    && chown tollgate:tollgate /data /run/tollgate /alerts \
    && chmod 700 /data /run/tollgate /alerts
COPY --from=build /out/tollgate /usr/local/bin/tollgate
COPY deploy/tollgate/entrypoint.sh /usr/local/bin/tollgate-entrypoint
# The entrypoint starts as root only to read the secrets and find the listen
# address, then drops to the tollgate user (setpriv).
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["bash", "-c", "a=$(cat /run/tollgate/listen) && exec 3<>/dev/tcp/${a%:*}/${a##*:} && printf 'GET /index.html HTTP/1.0\\r\\n\\r\\n' >&3 && head -n1 <&3 | grep -q ' 200 '"]
ENTRYPOINT ["/usr/local/bin/tollgate-entrypoint"]
