#!/bin/sh
# Install a released terraform-provider-hyperv binary into Terraform's implied
# local plugin mirror so `terraform init` finds it without any CLI config.
#
#   sh scripts/install.sh            # latest release
#   sh scripts/install.sh v0.2.0     # specific release
#
# Env overrides:
#   PLUGIN_DIR   default ~/.terraform.d/plugins
#   REPO         default ryanturner03/hyperv-terraform-provider
#   BASE_URL     override the download URL (testing / internal mirrors)
set -eu

REPO="${REPO:-ryanturner03/hyperv-terraform-provider}"
PROVIDER_ADDR="registry.terraform.io/ryan/hyperv"
PLUGIN_DIR="${PLUGIN_DIR:-$HOME/.terraform.d/plugins}"
VERSION="${1:-}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: $1 is required" >&2; exit 1; }; }
need curl; need unzip

if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || { echo "error: could not determine latest release" >&2; exit 1; }
fi
VER="${VERSION#v}"

case "$(uname -s)" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "error: unsupported OS $(uname -s); on Windows use scripts/install.ps1" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "error: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

BASE="${BASE_URL:-https://github.com/$REPO/releases/download/v$VER}"
ZIP="terraform-provider-hyperv_${VER}_${OS}_${ARCH}.zip"
SUMS="terraform-provider-hyperv_${VER}_SHA256SUMS"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $ZIP"
curl -fsSL -o "$TMP/$ZIP" "$BASE/$ZIP"
curl -fsSL -o "$TMP/$SUMS" "$BASE/$SUMS"

echo "Verifying checksum"
EXPECTED=$(grep " $ZIP\$" "$TMP/$SUMS" | cut -d' ' -f1)
[ -n "$EXPECTED" ] || { echo "error: $ZIP not listed in $SUMS" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMP/$ZIP" | cut -d' ' -f1)
else
  ACTUAL=$(shasum -a 256 "$TMP/$ZIP" | cut -d' ' -f1)
fi
[ "$EXPECTED" = "$ACTUAL" ] || { echo "error: checksum mismatch for $ZIP" >&2; exit 1; }

DEST="$PLUGIN_DIR/$PROVIDER_ADDR/$VER/${OS}_${ARCH}"
mkdir -p "$DEST"
unzip -oq "$TMP/$ZIP" -d "$DEST"
chmod +x "$DEST"/terraform-provider-hyperv_v*

cat <<MSG
Installed terraform-provider-hyperv v$VER to
  $DEST

Use it with:

  terraform {
    required_providers {
      hyperv = {
        source  = "ryan/hyperv"
        version = "$VER"
      }
    }
  }

Then run: terraform init
MSG
