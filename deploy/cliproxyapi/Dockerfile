# Builds CLIProxyAPI from the pinned upstream commit without cgo (ADR-0014).
# The pin lives only in the ARG below; `make cpa` reads it from here, and
# UPSTREAM.md describes the update. The upstream module has no replace
# directives; sum.golang.org verifies the downloaded source checksum.
# Base images carry tag and multi-arch index digest (#59); update: docs/DEPLOY.md.
FROM golang:1.26-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS builder
ARG CPA_VERSION=8ef43e4df3b216a42493105d31c2873b69191473
# MIT source stays in the build only; ADR-0016 adds the reviewed refresh barrier,
# ADR-0020 the catalog overlay for claude-haiku-5-5.
COPY bridge /bridge
RUN dir=$(go mod download -json github.com/router-for-me/CLIProxyAPI/v8@${CPA_VERSION} \
        | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$/\1/p') \
    && test -n "$dir" \
    && cp -a "$dir" /src-cpa && chmod -R u+w /src-cpa \
    && /bridge/apply.sh /src-cpa \
    && cd /src-cpa \
    && go test -race -run '^TestTollgatePause' ./sdk/cliproxy/auth \
    && go test -race -run '^TestTollgateHaiku55' ./internal/registry \
    && mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/cli-proxy-api ./cmd/server \
    && cp LICENSE /out/LICENSE

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251
# iptables: the entrypoint limits egress to Anthropic (ADR-0017); setpriv
# comes with util-linux from the base image.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates iptables \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/cli-proxy-api /usr/local/bin/cli-proxy-api
COPY --from=builder /out/LICENSE /usr/share/doc/cli-proxy-api/LICENSE
COPY render-config.sh /usr/local/bin/render-config.sh
COPY config.yaml /etc/cliproxyapi/config.template.yaml
COPY entrypoint.sh /usr/local/bin/cpa-entrypoint
# CLIProxyAPI runs as cpa; the named volume inherits the ownership of
# /data/auth. No USER: the entrypoint starts as root only to set the egress
# rules and then drops to cpa without capabilities.
RUN useradd --system --uid 10001 --home-dir /nonexistent --shell /usr/sbin/nologin cpa \
    && mkdir -p /data/auth /run/cpa \
    && chown cpa:cpa /data/auth /run/cpa
# debian-slim has no curl; bash /dev/tcp asks the public /healthz endpoint.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/8317 && printf 'GET /healthz HTTP/1.0\\r\\n\\r\\n' >&3 && grep -q '\"ok\"' <&3"]
ENTRYPOINT ["/usr/local/bin/cpa-entrypoint"]
