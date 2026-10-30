#!/usr/bin/env bash
# Build a Debian/Ubuntu package for Local Agent.
#
#   ./build/linux/package-deb.sh [version]
#
# Expects build/bin/localagent to exist (run `wails build -tags webkit2_41`
# first) and build/linux/icons/localagent-*.png (rendered from appicon.svg).
# Output: build/bin/localagent_<version>_<arch>.deb
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${1:-0.1.0}"
ARCH="$(dpkg --print-architecture)"
PKG="localagent"
BIN="$ROOT/build/bin/$PKG"
STAGE="$ROOT/build/linux/deb/${PKG}_${VERSION}_${ARCH}"
OUT="$ROOT/build/bin/${PKG}_${VERSION}_${ARCH}.deb"

[ -x "$BIN" ] || { echo "missing $BIN — run: wails build -tags webkit2_41" >&2; exit 1; }

rm -rf "$STAGE"
install -Dm755 "$BIN" "$STAGE/usr/bin/$PKG"
install -Dm644 "$ROOT/build/linux/$PKG.desktop" "$STAGE/usr/share/applications/$PKG.desktop"
for png in "$ROOT"/build/linux/icons/$PKG-*.png; do
  size="$(basename "$png" .png)"; size="${size##*-}"
  install -Dm644 "$png" "$STAGE/usr/share/icons/hicolor/${size}x${size}/apps/$PKG.png"
done
install -Dm644 "$ROOT/build/appicon.svg" "$STAGE/usr/share/icons/hicolor/scalable/apps/$PKG.svg"
install -Dm644 "$ROOT/build/linux/icons/$PKG-128.png" "$STAGE/usr/share/pixmaps/$PKG.png"
install -Dm644 "$ROOT/README.md" "$STAGE/usr/share/doc/$PKG/README.md"
install -Dm644 /dev/stdin "$STAGE/usr/share/doc/$PKG/copyright" <<EOF
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: Local Agent
Files: *
Copyright: $(date +%Y) Ananthu Krishnan
License: MIT
EOF

INSTALLED_KB="$(du -sk "$STAGE/usr" | cut -f1)"
mkdir -p "$STAGE/DEBIAN"
cat > "$STAGE/DEBIAN/control" <<EOF
Package: $PKG
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: Ananthu Krishnan <ananthu.krishnan@aurex.ai>
Installed-Size: $INSTALLED_KB
Depends: libgtk-3-0t64 | libgtk-3-0, libwebkit2gtk-4.1-0, libjavascriptcoregtk-4.1-0, libglib2.0-bin
Recommends: libnotify-bin, google-chrome-stable | chromium | chromium-browser | microsoft-edge-stable
Suggests: brightnessctl
Homepage: https://github.com/aurex-ai/localagent
Description: Local AI agent that operates your laptop with an Ollama model
 Local Agent is a desktop chat app. A model running locally in Ollama
 answers questions about the machine and carries out tasks: running
 commands, managing files, opening apps, reading email and websites in
 its own Chrome profile, and running prompts on a schedule. Anything
 that changes the system asks for approval first.
 .
 Requires Ollama (https://ollama.com) with a tool-capable model such as
 qwen3.5:2b. Email and web features need Google Chrome, Chromium or Edge.
EOF

cat > "$STAGE/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "configure" ]; then
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/share/applications || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
exit 0
EOF
cat > "$STAGE/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/share/applications || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
exit 0
EOF
chmod 755 "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/postrm"

rm -f "$OUT"
dpkg-deb --build --root-owner-group -Zxz "$STAGE" "$OUT"
echo
echo "Built $OUT"
dpkg-deb --info "$OUT" | sed -n '1,12p'
