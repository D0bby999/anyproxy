#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Compiles the macOS NetworkExtension (.systemextension) into AnyProxy.app.
set -euo pipefail

APP_PATH="${1:-build/bin/AnyProxy.app}"
ARCH="${2:-$(uname -m | sed 's/amd64/x86_64/')}"
SIGN_ID="${SIGN_ID:--}"
EXT_ID="dev.anyproxy.app.extension"
EXT_DIR="$APP_PATH/Contents/Library/SystemExtensions/$EXT_ID.systemextension"
EXT_MACOS="$EXT_DIR/Contents/MacOS"

mkdir -p "$EXT_MACOS"
cp macos/extension/Info.plist "$EXT_DIR/Contents/Info.plist"
EXT_BUILD_NUM="${GITHUB_RUN_NUMBER:-$(date +%s)}"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $EXT_BUILD_NUM" "$EXT_DIR/Contents/Info.plist"

if [ "$ARCH" = "universal" ]; then
    xcrun swiftc -target arm64-apple-macos11.0 -module-name AnyProxyExtension -O \
        -o "$EXT_MACOS/$EXT_ID.arm64" macos/extension/*.swift
    xcrun swiftc -target x86_64-apple-macos11.0 -module-name AnyProxyExtension -O \
        -o "$EXT_MACOS/$EXT_ID.x86_64" macos/extension/*.swift
    lipo -create -output "$EXT_MACOS/$EXT_ID" "$EXT_MACOS/$EXT_ID.arm64" "$EXT_MACOS/$EXT_ID.x86_64"
    rm -f "$EXT_MACOS/$EXT_ID.arm64" "$EXT_MACOS/$EXT_ID.x86_64"
else
    SWIFT_ARCH="$(echo "$ARCH" | sed 's/amd64/x86_64/')"
    xcrun swiftc -target "${SWIFT_ARCH}-apple-macos11.0" -module-name AnyProxyExtension -O \
        -o "$EXT_MACOS/$EXT_ID" macos/extension/*.swift
fi

# Embed Provisioning Profiles when provided (required by macOS SIP/AMFI to load System Extensions).
if [ -n "${EXT_PROVISION_PROFILE:-}" ] && [ -f "${EXT_PROVISION_PROFILE}" ]; then
    cp "${EXT_PROVISION_PROFILE}" "$EXT_DIR/Contents/embedded.provisionprofile"
fi
if [ -n "${APP_PROVISION_PROFILE:-}" ] && [ -f "${APP_PROVISION_PROFILE}" ]; then
    cp "${APP_PROVISION_PROFILE}" "$APP_PATH/Contents/embedded.provisionprofile"
fi

SIGN_OPTS=()
if [[ "$SIGN_ID" == *"Developer ID"* ]]; then
    SIGN_OPTS+=(--options runtime --timestamp)
fi

# Only attach NetworkExtension / SystemExtension entitlements when provisioning profiles are embedded,
# so local dev builds without provisioning profiles still launch normally (falling back to macos-sysproxy).
if [ -f "$EXT_DIR/Contents/embedded.provisionprofile" ] && [ -f "$APP_PATH/Contents/embedded.provisionprofile" ]; then
    codesign --force ${SIGN_OPTS[@]+"${SIGN_OPTS[@]}"} --entitlements macos/extension/extension.entitlements --sign "$SIGN_ID" "$EXT_DIR"
    codesign --force --deep ${SIGN_OPTS[@]+"${SIGN_OPTS[@]}"} --entitlements build/darwin/AnyProxy.entitlements --sign "$SIGN_ID" "$APP_PATH"
else
    codesign --force ${SIGN_OPTS[@]+"${SIGN_OPTS[@]}"} --sign "$SIGN_ID" "$EXT_DIR"
    codesign --force --deep ${SIGN_OPTS[@]+"${SIGN_OPTS[@]}"} --sign "$SIGN_ID" "$APP_PATH"
fi
