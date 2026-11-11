# SPDX-License-Identifier: AGPL-3.0-or-later
WAILS   ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
APP     := build/bin/AnyProxy.app
VERSION ?= $(shell sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
LDFLAGS := -s -w -X github.com/D0bby999/anyproxy/internal/update.Current=$(VERSION)
SIGN_ID ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep -q '"AnyProfile Dev"' && echo 'AnyProfile Dev' || echo -)
SIGN_FLAGS ?=

.PHONY: dev build build-windows test lint

dev:
	$(WAILS) dev

build:
	$(WAILS) build -clean -platform darwin/arm64 -ldflags "$(LDFLAGS)"
	codesign --force --deep $(SIGN_FLAGS) --sign "$(SIGN_ID)" $(APP)
	codesign --verify --deep --strict $(APP)

build-windows:
	mkdir -p build/bin
	$(WAILS) build -platform windows/amd64 -nsis -ldflags "$(LDFLAGS)"

test:
	go test -race ./internal/... -count=1
	cd frontend && npm run typecheck && npm test

lint:
	gofmt -l . | (! grep .) || (echo "gofmt needed" >&2; exit 1)
	go vet ./...
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...

