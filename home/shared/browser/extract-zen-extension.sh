#!/usr/bin/env bash

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

if [ $# -ne 1 ]; then
    echo "Usage: $0 <firefox-addon-download-link>"
    echo ""
    echo "Example:"
    echo "  $0 'https://addons.mozilla.org/firefox/downloads/file/4391368/wappalyzer-6.10.70.xpi'"
    exit 1
fi

DOWNLOAD_LINK="$1"

echo -e "${BLUE}Processing extension...${NC}"

# Extract the plugin slug
PLUGIN_SLUG=$(echo "$DOWNLOAD_LINK" \
    | sed -E 's|https://addons.mozilla.org/firefox/downloads/file/[0-9]+/([^/]+)-[^/]+\.xpi|\1|' \
    | tr '_' '-')

echo -e "${BLUE}Plugin slug: ${GREEN}$PLUGIN_SLUG${NC}"

# Generate the latest.xpi URL
LATEST_URL="https://addons.mozilla.org/firefox/downloads/latest/$PLUGIN_SLUG/latest.xpi"

echo -e "${BLUE}Downloading from: ${GREEN}$LATEST_URL${NC}"

# Create temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Download the extension
curl -sL -o "$TEMP_DIR/extension.xpi" "$LATEST_URL"

# Extract the extension
unzip -q "$TEMP_DIR/extension.xpi" -d "$TEMP_DIR/extension"

# Get the extension ID
EXTENSION_ID=$(jq -r '.browser_specific_settings.gecko.id' "$TEMP_DIR/extension/manifest.json")

if [ "$EXTENSION_ID" = "null" ] || [ -z "$EXTENSION_ID" ]; then
    echo -e "${YELLOW}Warning: Could not find extension ID in manifest.json${NC}"
    echo "Trying alternative locations..."
    EXTENSION_ID=$(jq -r '.applications.gecko.id // empty' "$TEMP_DIR/extension/manifest.json")
fi

if [ -z "$EXTENSION_ID" ] || [ "$EXTENSION_ID" = "null" ]; then
    echo "Error: Could not extract extension ID from manifest.json"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ Success!${NC}"
echo ""
echo -e "${BLUE}Add this line to your ExtensionSettings:${NC}"
echo ""
echo "      \"$EXTENSION_ID\" = \"$PLUGIN_SLUG\";"
echo ""
echo -e "${BLUE}(Full context - you likely already have this setup:)${NC}"
echo ""
cat << EOF
  programs.zen-browser.policies = let
    mkExtensionSettings = builtins.mapAttrs (_: pluginId: {
      install_url = "https://addons.mozilla.org/firefox/downloads/latest/\${pluginId}/latest.xpi";
      installation_mode = "force_installed";
    });
  in {
    ExtensionSettings = mkExtensionSettings {
      "$EXTENSION_ID" = "$PLUGIN_SLUG";  # ← Add your new extensions here
    };
  };
EOF

