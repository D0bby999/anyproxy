# SPDX-License-Identifier: AGPL-3.0-or-later
SHELL   := /bin/bash
WAILS   ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
APP     := build/bin/AnyProxy.app
ARCH    ?= $(shell uname -m | sed 's/x86_64/amd64/')
DMG     := dist/AnyProxy-darwin-$(ARCH).dmg
ZIP     := dist/AnyProxy-darwin-$(ARCH).zip
VERSION ?= $(shell sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
LDFLAGS := -s -w -X github.com/D0bby999/anyproxy/internal/update.Current=$(VERSION)
DEV_ID  ?= $(shell security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)
SIGN_ID ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep -q '"AnyProfile Dev"' && echo 'AnyProfile Dev' || echo -)
SIGN_FLAGS ?= $(if $(findstring Developer ID,$(SIGN_ID)),--options runtime --timestamp,)
NOTARY_PROFILE ?=
NOTARY_ENV     ?=

.PHONY: dev build sign-macos package-macos verify-macos-sig notarize-macos build-windows test lint

dev:
	$(WAILS) dev

build:
	$(WAILS) build -clean -platform darwin/$(ARCH) -ldflags "$(LDFLAGS)"
	codesign --force --deep $(SIGN_FLAGS) --sign "$(SIGN_ID)" $(APP)
	codesign --verify --deep --strict $(APP)

sign-macos:
	@test -n "$(DEV_ID)" || (echo "error: no Developer ID Application certificate found in Keychain" >&2; exit 1)
	$(WAILS) build -clean -platform darwin/$(ARCH) -ldflags "$(LDFLAGS)"
	codesign --force --deep --options runtime --timestamp --sign "$(DEV_ID)" $(APP)
	codesign --verify --deep --strict $(APP)

verify-macos-sig:
	@bad=0; \
	while IFS= read -r -d '' f; do \
		file -b "$$f" | grep -q 'Mach-O' || continue; \
		sig="$$(codesign -dv --verbose=4 "$$f" 2>&1)"; \
		case "$$sig" in \
			*"Authority=Developer ID Application:"*Timestamp=*) ;; \
			*) echo "not signed for distribution: $$f" >&2; bad=1 ;; \
		esac; \
	done < <(find $(APP) -type f -print0); \
	exit "$$bad"

package-macos: sign-macos verify-macos-sig
	xattr -cr $(APP)
	mkdir -p dist/dmg-root
	rm -rf dist/dmg-root/*
	cp -R $(APP) dist/dmg-root/
	ln -s /Applications dist/dmg-root/Applications
	hdiutil create -volname "AnyProxy" -srcfolder dist/dmg-root -ov -format UDZO "$(DMG)"
	rm -rf dist/dmg-root
	codesign --force --timestamp --sign "$(DEV_ID)" "$(DMG)"

notarize-macos: sign-macos verify-macos-sig
	@set -euo pipefail; \
	if [ -n "$(NOTARY_ENV)" ]; then set -a; source "$(NOTARY_ENV)"; set +a; fi; \
	NOTARY_ARGS=(); \
	if [ -n "$(NOTARY_PROFILE)" ]; then \
		NOTARY_ARGS=(--keychain-profile "$(NOTARY_PROFILE)"); \
	elif [ -n "$${APPLE_KEY:-}" ] && [ -n "$${APPLE_KEY_ID:-}" ]; then \
		NOTARY_ARGS=(--key "$$APPLE_KEY" --key-id "$$APPLE_KEY_ID"); \
		if [ -n "$${APPLE_ISSUER:-}" ]; then NOTARY_ARGS+=(--issuer "$$APPLE_ISSUER"); fi; \
	else \
		echo "error: set NOTARY_PROFILE=<profile>, NOTARY_ENV=<notary.env>, or APPLE_KEY/APPLE_KEY_ID[/APPLE_ISSUER]" >&2; \
		exit 1; \
	fi; \
	mkdir -p dist/dmg-root; \
	rm -rf dist/dmg-root/* "$(ZIP)" "$(DMG)"; \
	xattr -cr $(APP); \
	ditto -c -k --norsrc --noextattr --keepParent $(APP) "$(ZIP)"; \
	xcrun notarytool submit "$(ZIP)" "$${NOTARY_ARGS[@]}" --wait --timeout 30m; \
	rm -f "$(ZIP)"; \
	xcrun stapler staple $(APP); \
	xcrun stapler validate $(APP); \
	cp -R $(APP) dist/dmg-root/; \
	ln -s /Applications dist/dmg-root/Applications; \
	hdiutil create -volname "AnyProxy" -srcfolder dist/dmg-root -ov -format UDZO "$(DMG)"; \
	rm -rf dist/dmg-root; \
	codesign --force --timestamp --sign "$(DEV_ID)" "$(DMG)"; \
	xcrun notarytool submit "$(DMG)" "$${NOTARY_ARGS[@]}" --wait --timeout 30m; \
	xcrun stapler staple "$(DMG)"; \
	xcrun stapler validate $(APP) "$(DMG)"; \
	spctl -a -vvv -t exec $(APP); \
	spctl -a -vv -t open --context context:primary-signature "$(DMG)"

build-windows:
	mkdir -p build/bin
	$(WAILS) build -platform windows/amd64 -nsis -ldflags "$(LDFLAGS)"

test:
	go test -race ./internal/... -count=1
	cd frontend && npm run typecheck && npm test

lint:
	@mkdir -p frontend/dist && test -f frontend/dist/index.html || touch frontend/dist/index.html
	gofmt -l . | (! grep .) || (echo "gofmt needed" >&2; exit 1)
	go vet ./...
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...

