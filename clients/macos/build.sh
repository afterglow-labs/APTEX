#!/bin/bash
set -euo pipefail
TASK_MACOS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_REPO_ROOT="$(cd "$TASK_MACOS_ROOT/../.." && pwd)"
TASK_OUTPUT="$TASK_REPO_ROOT/dist/macos"
TASK_ICON="$TASK_REPO_ROOT/clients/ios/APTEX/Resources/Assets.xcassets/AppIcon.appiconset/AppIcon.png"
[ "$(uname -s)" = Darwin ] || { echo 'Build the Mac client on macOS or use the macOS CI artifact.' >&2; exit 1; }
[ -f "$TASK_MACOS_ROOT/Package.swift" ] && [ -f "$TASK_REPO_ROOT/LICENSE" ] || exit 1
[ -f "$TASK_ICON" ] || { echo "Shared APTEX app icon is missing: $TASK_ICON" >&2; exit 1; }
for tool in swift xcrun lipo sips iconutil codesign ditto; do command -v "$tool" >/dev/null || { echo "Missing macOS build tool: $tool" >&2; exit 1; }; done
mkdir -p "$TASK_OUTPUT"
TASK_TEMP="$(mktemp -d "$TASK_OUTPUT/.aptex-build.XXXXXX")"
trap 'rm -rf "$TASK_TEMP"' EXIT
TASK_APP="$TASK_TEMP/APTEX.app"
mkdir -p "$TASK_APP/Contents/MacOS" "$TASK_APP/Contents/Resources"
for task_arch in arm64 x86_64; do
  swift build --package-path "$TASK_MACOS_ROOT" --configuration release --arch "$task_arch" --scratch-path "$TASK_TEMP/$task_arch"
  task_bin="$(swift build --package-path "$TASK_MACOS_ROOT" --configuration release --arch "$task_arch" --scratch-path "$TASK_TEMP/$task_arch" --show-bin-path)"
  test -x "$task_bin/APTEXClient"
  cp "$task_bin/APTEXClient" "$TASK_TEMP/APTEX-$task_arch"
done
lipo -create "$TASK_TEMP/APTEX-arm64" "$TASK_TEMP/APTEX-x86_64" -output "$TASK_APP/Contents/MacOS/APTEX"
cp "$TASK_MACOS_ROOT/Info.plist" "$TASK_APP/Contents/Info.plist"
cp "$TASK_REPO_ROOT/LICENSE" "$TASK_APP/Contents/Resources/LICENSE"
TASK_ICONSET="$TASK_TEMP/APTEX.iconset"
mkdir -p "$TASK_ICONSET"
for points in 16 32 128 256 512; do
  sips -z "$points" "$points" "$TASK_ICON" --out "$TASK_ICONSET/icon_${points}x${points}.png" >/dev/null
  pixels=$((points * 2))
  if [ "$pixels" -eq 1024 ]; then
    cp "$TASK_ICON" "$TASK_ICONSET/icon_512x512@2x.png"
  else
    sips -z "$pixels" "$pixels" "$TASK_ICON" --out "$TASK_ICONSET/icon_${points}x${points}@2x.png" >/dev/null
  fi
done
iconutil -c icns "$TASK_TEMP/APTEX.iconset" -o "$TASK_APP/Contents/Resources/APTEX.icns"
TASK_IDENTITY="${APTEX_SIGN_IDENTITY:--}"
if [ "$TASK_IDENTITY" = '-' ]; then
  codesign --force --deep --sign - --timestamp=none "$TASK_APP"
else
  codesign --force --deep --options runtime --timestamp --sign "$TASK_IDENTITY" "$TASK_APP"
fi
codesign --verify --deep --strict "$TASK_APP"
TASK_ARCHS="$(lipo -archs "$TASK_APP/Contents/MacOS/APTEX")"
for task_arch in arm64 x86_64; do
  case " $TASK_ARCHS " in
    *" $task_arch "*) ;;
    *) echo "Mac client build is missing $task_arch: $TASK_ARCHS" >&2; exit 1 ;;
  esac
done
if [ -e "$TASK_OUTPUT/APTEX.app" ]; then
  test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$TASK_OUTPUT/APTEX.app/Contents/Info.plist")" = 'com.afterglowlabs.aptex.client' || { echo 'Refusing to replace an unrelated application.' >&2; exit 1; }
  rm -rf "$TASK_OUTPUT/APTEX.app"
fi
mv "$TASK_APP" "$TASK_OUTPUT/APTEX.app"
ditto -c -k --sequesterRsrc --keepParent "$TASK_OUTPUT/APTEX.app" "$TASK_OUTPUT/APTEX-macOS-universal.zip"
(cd "$TASK_OUTPUT" && shasum -a 256 APTEX-macOS-universal.zip > SHA256SUMS.txt)
echo "Mac client ready: $TASK_OUTPUT/APTEX-macOS-universal.zip"
