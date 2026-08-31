VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PLATFORMS := darwin/arm64 darwin/amd64 linux/arm64 linux/amd64
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o golem ./cmd/golem

dist:
	rm -rf dist
	mkdir -p dist
	for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		name=golem_$(VERSION)_$${goos}_$${goarch}; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -ldflags "$(LDFLAGS)" -o dist/$$name/golem ./cmd/golem; \
		cp LICENSE dist/$$name/; \
		COPYFILE_DISABLE=1 tar -C dist -czf dist/$$name.tar.gz $$name; \
		rm -rf dist/$$name; \
	done
	cd dist && { command -v sha256sum >/dev/null && sha256sum *.tar.gz || shasum -a 256 *.tar.gz; } > checksums.txt
