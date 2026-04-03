#!/bin/bash
# Install script for Linux Monitor
# This script builds the application, installs it to /usr/local/bin, and creates a desktop entry for easy access.
# Usage: Run this script from the root of the project directory.
# Note: You may need to run this script with sudo if you don't have permission to write to /usr/local/bin.
# Example: sudo ./install.sh

set -e

APP_NAME="linux-monitor"
INSTALL_PATH="/usr/local/bin/$APP_NAME"

echo "🔧 Building..."
go build -o $APP_NAME

echo "📦 Installing..."
sudo cp $APP_NAME $INSTALL_PATH
sudo chmod +x $INSTALL_PATH

echo "🖥 Creating desktop entry..."
sudo tee /usr/share/applications/$APP_NAME.desktop > /dev/null <<EOF
[Desktop Entry]
Name=Linux Monitor
Exec=$INSTALL_PATH
Icon=utilities-system-monitor
Type=Application
Categories=System;
Terminal=false
EOF

echo "✅ Done!"
echo "👉 Start with: $APP_NAME"