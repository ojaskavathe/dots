set shell := ["bash", "-c"]

default:
    @just --list --unsorted

alias fmt := format

format:
    find . -name '*.nix' -not -path './secrets/*' | xargs nixfmt
    find . -name '*.lua' | xargs stylua

check:
    find . -name '*.nix' -not -path './secrets/*' | xargs nixfmt --check
    find . -name '*.lua' | xargs stylua --check

secrets-edit:
    sops secrets/hosts/common/secrets.yaml

secrets-view:
    sops -d secrets/hosts/common/secrets.yaml
