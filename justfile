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

# build system + home closures without activating, so drs/hms are store-hits
prebuild:
    #!/usr/bin/env bash
    # --out-link keeps a gc root, so an intervening `nix store gc` can't undo the work
    set -euo pipefail
    host=$(hostname -s)
    user=$(whoami)
    case "$(uname -s)" in
      Darwin) system=".#darwinConfigurations.${host}.system" ;;
      Linux) system=".#nixosConfigurations.${host}.config.system.build.toplevel" ;;
      *) echo "unsupported platform: $(uname -s)" >&2; exit 1 ;;
    esac
    nix build --out-link .result-system "$system"
    nix build --out-link .result-home ".#homeConfigurations.\"${user}@${host}\".activationPackage"

secrets-edit:
    sops secrets/hosts/common/secrets.yaml

secrets-view:
    sops -d secrets/hosts/common/secrets.yaml

update-claude:
    #!/usr/bin/env bash
    set -euo pipefail
    base="https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"
    version=$(curl -sf "$base/latest")
    current=$(jq -r '.version' modules/home/claude-version.json)
    if [[ "$version" == "$current" ]]; then
      echo "claude: already up to date ($version)"
      exit 0
    fi
    manifest=$(curl -sf "$base/$version/manifest.json")
    hex_to_sri() { echo "sha256-$(echo "$1" | xxd -r -p | base64)"; }
    darwin=$(hex_to_sri "$(echo "$manifest" | jq -r '.platforms."darwin-arm64".checksum')")
    linux=$(hex_to_sri "$(echo "$manifest" | jq -r '.platforms."linux-x64".checksum')")
    jq -n --arg v "$version" --arg d "$darwin" --arg l "$linux" \
      '{version: $v, hashes: {"aarch64-darwin": $d, "x86_64-linux": $l}}' \
      > modules/home/claude-version.json
    echo "claude: updated $current -> $version"

update-codex:
    #!/usr/bin/env bash
    set -euo pipefail
    # codex is packaged from the npm launcher @openai/codex (see codex.nix): the
    # standalone tarball omits codex-code-mode-host, which code mode needs.
    version=$(npm view @openai/codex version)
    current=$(jq -r '.version' modules/home/codex-version.json)
    if [[ "$version" == "$current" ]]; then
      echo "codex: already up to date ($version)"
      exit 0
    fi
    sri() { nix hash convert --hash-algo sha256 --to sri "$1"; }
    # src hash: the launcher tarball, hashed unpacked to match fetchzip
    src_hash=$(sri "$(nix-prefetch-url --unpack --type sha256 "https://registry.npmjs.org/@openai/codex/-/codex-${version}.tgz" 2>/dev/null | tail -1)")
    # regenerate the pinned lockfile, then hash its dependency closure
    tmp=$(mktemp -d)
    ( cd "$tmp" && npm i --package-lock-only "@openai/codex@${version}" >/dev/null 2>&1 )
    cp "$tmp/package-lock.json" modules/home/codex-package-lock.json
    rm -rf "$tmp"
    npm_deps=$(sri "$(nix run nixpkgs#prefetch-npm-deps -- modules/home/codex-package-lock.json)")
    jq -n --arg v "$version" --arg s "$src_hash" --arg n "$npm_deps" \
      '{version: $v, srcHash: $s, npmDepsHash: $n}' \
      > modules/home/codex-version.json
    echo "codex: updated $current -> $version"

update-grok:
    #!/usr/bin/env bash
    set -euo pipefail
    base="https://storage.googleapis.com/grok-build-public-artifacts/cli"
    # the bucket has no release index; `stable` holds the current version string
    version=$(curl -sf "$base/stable")
    current=$(jq -r '.version' modules/home/grok-version.json)
    if [[ "$version" == "$current" ]]; then
      echo "grok: already up to date ($version)"
      exit 0
    fi
    sri() { nix hash convert --hash-algo sha256 --to sri "$(nix-prefetch-url --type sha256 "$1" 2>/dev/null | tail -1)"; }
    darwin=$(sri "${base}/grok-${version}-macos-aarch64")
    linux=$(sri "${base}/grok-${version}-linux-x86_64")
    jq -n --arg v "$version" --arg d "$darwin" --arg l "$linux" \
      '{version: $v, hashes: {"aarch64-darwin": $d, "x86_64-linux": $l}}' \
      > modules/home/grok-version.json
    echo "grok: updated $current -> $version"

# bump every pinned coding-agent CLI
update-agents: update-claude update-codex update-grok
