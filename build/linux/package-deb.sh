#!/usr/bin/env bash
# Builds the Modbus Studio .deb package.
#
#   build/linux/package-deb.sh VERSION BINARY DEPENDS OUTPUT.deb
#
# VERSION is the app version (e.g. 0.3.0 or 0.3.0-beta.1), BINARY the
# built executable, DEPENDS the WebKitGTK package for the target Ubuntu
# (libwebkit2gtk-4.0-37 or libwebkit2gtk-4.1-0).
set -euo pipefail

VERSION="$1"; BINARY="$2"; WEBKIT="$3"; OUT="$4"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(mktemp -d)"
chmod 755 "$ROOT"  # mktemp makes it 0700; the package root must be 0755
trap 'rm -rf "$ROOT"' EXIT

# Debian sorts "~" before anything, so 0.3.0~beta.1 < 0.3.0.
# Conflicts/Replaces: installing this removes the old "appmodbus" package
# (the app's name up to v2.0.0) instead of installing beside it.
DEBVER="${VERSION/-/\~}"

install -Dm755 "$BINARY" "$ROOT/opt/ModbusStudio/ModbusStudio"
mkdir -p "$ROOT/usr/bin"
ln -s /opt/ModbusStudio/ModbusStudio "$ROOT/usr/bin/modbus-studio"
install -Dm644 "$HERE/modbus-studio.desktop" "$ROOT/usr/share/applications/modbus-studio.desktop"
install -Dm644 "$HERE/../appicon.png" "$ROOT/usr/share/pixmaps/modbus-studio.png"

SIZE=$(du -sk "$ROOT" | cut -f1)
mkdir -p "$ROOT/DEBIAN"
cat > "$ROOT/DEBIAN/control" <<CONTROL
Package: modbus-studio
Version: $DEBVER
Section: electronics
Priority: optional
Architecture: amd64
Installed-Size: $SIZE
Depends: libgtk-3-0, $WEBKIT
Conflicts: appmodbus
Replaces: appmodbus
Maintainer: Modbus Studio <elmnaveen@users.noreply.github.com>
Homepage: https://github.com/elmnaveen/AppMODBUS
Description: Modbus master and slave simulator
 Modbus Studio polls Modbus devices over RTU (serial), Modbus TCP/IP and RTU
 over TCP/UDP, with register tags, device profiles, recording and a
 built-in slave simulator.
 .
 To use serial ports, add your user to the dialout group:
   sudo usermod -aG dialout \$USER   (then log out and in again)
CONTROL

cat > "$ROOT/DEBIAN/postinst" <<'POSTINST'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi
POSTINST
chmod 755 "$ROOT/DEBIAN/postinst"
cp "$ROOT/DEBIAN/postinst" "$ROOT/DEBIAN/postrm"

# -Zxz keeps the package readable by dpkg on Ubuntu 18.04 (no zstd).
dpkg-deb --root-owner-group -Zxz --build "$ROOT" "$OUT" >/dev/null
echo "built $OUT ($DEBVER, depends $WEBKIT)"
