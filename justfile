set shell := ["bash", "-c"]

default:
    @just --list --unsorted

alias fmt := format

format:
    find . -name '*.nix' -not -path './secrets/*' | xargs nixfmt
    find . -name '*.lua' | xargs stylua
    find . -name '*.sh' | xargs shfmt -w -i 2
    find . -name '*.md' | xargs prettier --write --prose-wrap always

check:
    find . -name '*.nix' -not -path './secrets/*' | xargs nixfmt --check
    find . -name '*.lua' | xargs stylua --check
    find . -name '*.sh' | xargs shfmt -d -i 2
    find . -name '*.md' | xargs prettier --check --prose-wrap always

secrets-edit:
    sops secrets/hosts/common/secrets.yaml

secrets-view:
    sops -d secrets/hosts/common/secrets.yaml

update-claude:
    #!/usr/bin/env bash
    set -euo pipefail
    base="https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"
    version=$(curl -sf "$base/latest")
    current=$(jq -r '.version' home/shared/claude-version.json)
    if [[ "$version" == "$current" ]]; then
      echo "already up to date ($version)"
      exit 0
    fi
    manifest=$(curl -sf "$base/$version/manifest.json")
    hex_to_sri() { echo "sha256-$(echo "$1" | xxd -r -p | base64)"; }
    darwin=$(hex_to_sri "$(echo "$manifest" | jq -r '.platforms."darwin-arm64".checksum')")
    linux=$(hex_to_sri "$(echo "$manifest" | jq -r '.platforms."linux-x64".checksum')")
    jq -n --arg v "$version" --arg d "$darwin" --arg l "$linux" \
      '{version: $v, hashes: {"aarch64-darwin": $d, "x86_64-linux": $l}}' \
      > home/shared/claude-version.json
    echo "updated $current -> $version"
