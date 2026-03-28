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
