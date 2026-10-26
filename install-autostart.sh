#!/usr/bin/env sh
# builds the app, installs it to ~/.local/bin and registers it to start on login
# tested on Fedora KDE
set -eu

cd "$(dirname "$0")"

BIN_NAME=steelseries-battery-tray
BIN_DIR="$HOME/.local/bin"
AUTOSTART_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/autostart"

go build -o "$BIN_NAME" .

mkdir -p "$BIN_DIR" "$AUTOSTART_DIR"
install -m 755 "$BIN_NAME" "$BIN_DIR/$BIN_NAME"

cat > "$AUTOSTART_DIR/$BIN_NAME.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=SteelSeries Battery Tray
Exec=$BIN_DIR/$BIN_NAME
X-GNOME-Autostart-enabled=true
EOF

echo "Installed $BIN_DIR/$BIN_NAME"
echo "Autostart entry: $AUTOSTART_DIR/$BIN_NAME.desktop"
