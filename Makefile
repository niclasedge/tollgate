.PHONY: build cpa web web-test test lint generate

# The CLIProxyAPI pin lives only in its Dockerfile (ADR-0014).
CPA_VERSION := $(shell sed -n 's/^ARG CPA_VERSION=//p' deploy/cliproxyapi/Dockerfile)

# Go-only build. A fresh checkout embeds a placeholder page (web/embed.go);
# run `make web build` for a binary with the real frontend.
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/tollgate ./cmd/tollgate

# Builds bin/cli-proxy-api from the pinned upstream commit, e.g. for the
# smoke test: TOLLGATE_CPA_BIN=$PWD/bin/cli-proxy-api go test -run Smoke ./internal/upstream/
cpa:
	@test -n "$(CPA_VERSION)" || { echo "ARG CPA_VERSION missing in deploy/cliproxyapi/Dockerfile"; exit 1; }
	mkdir -p bin
	GOBIN=$(CURDIR)/bin CGO_ENABLED=0 go install -ldflags="-s -w" github.com/router-for-me/CLIProxyAPI/v8/cmd/server@$(CPA_VERSION)
	mv bin/server bin/cli-proxy-api

# Builds the frontend into web/dist (embedded by `make build`). Empties dist
# first but keeps the versioned .gitkeep (rm does not glob dotfiles).
# pnpm runs inside web/ so corepack picks the pinned version from
# web/package.json (from the repo root it would fetch the latest pnpm).
web:
	cd web && pnpm install --frozen-lockfile
	rm -rf web/dist/*
	cd web && pnpm build

web-test:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm check
	cd web && pnpm test

# The race detector needs a C toolchain; do not disable cgo here.
test: web-test
	go test -race ./...

# Fails on unformatted files or vet findings. Checks only the files of
# packages in this module.
lint:
	@files=$$(go list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .XTestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...); \
	if [ -n "$$files" ]; then \
		unformatted=$$(gofmt -l $$files); \
		if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi; \
	fi
	go vet ./...

# Pinned Go tool; without API packages generation is a no-op.
generate:
	@if grep -Eq '^packages:[[:space:]]*\[\][[:space:]]*$$' tygo.yaml; then \
		go tool tygo --version && echo "Keine API-Typen konfiguriert; Generierung ist ein No-op."; \
	else \
		go tool tygo generate; \
	fi
