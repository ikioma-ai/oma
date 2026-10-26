.PHONY: build test integration lint lint-actions coverage dist test-release installer-health

PLATFORMS = linux darwin
ARCHITECTURES = amd64 arm64
VERSION := $(shell git describe --tags --always)
LDFLAGS := -ldflags "-s -w -X 'github.com/ikioma-ai/oma/internal/version.Version=$(VERSION)'"

TEST_RELEASE_TAG = v0.0.1-test

build:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o bin/ ./cmd/...

build-all:
	@for os in $(PLATFORMS); do \
		for arch in $(ARCHITECTURES); do \
			echo "Building for $$os/$$arch..."; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath $(LDFLAGS) -o bin/$$os-$$arch/ ./cmd/...; \
		done; \
	done

test:
	go test ./internal/...

integration:
	go test -v -count=1 -parallel 16 -timeout 30m ./integration/...

check:
	go vet ./...
	govulncheck ./...

lint:
	golangci-lint run

lint-actions:
	actionlint
	zizmor .

coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out

dist: build-all
	mkdir -p dist
	@for os in $(PLATFORMS); do \
		for arch in $(ARCHITECTURES); do \
			cp bin/$$os-$$arch/oma dist/oma-$$os-$$arch; \
		done; \
	done
	cd dist && if command -v sha256sum >/dev/null 2>&1; then sha256sum oma-* > checksums.txt; else shasum -a 256 oma-* > checksums.txt; fi

test-release:
	-gh release delete $(TEST_RELEASE_TAG) --yes --cleanup-tag
	git tag -f $(TEST_RELEASE_TAG)
	git push origin $(TEST_RELEASE_TAG) --force

installer-health:
	@test -n "$(OMA_INSTALLER_URL)" || (echo "OMA_INSTALLER_URL is required" >&2; exit 1)
	@curl -fsS "$(OMA_INSTALLER_URL)/up" >/dev/null
	@curl -fsS "$(OMA_INSTALLER_URL)/apps.json" >/dev/null
	@curl -fsS "$(OMA_INSTALLER_URL)/" | grep -q "REPO='ikioma-ai/oma'"
	@echo "Installer health OK: $(OMA_INSTALLER_URL)"
