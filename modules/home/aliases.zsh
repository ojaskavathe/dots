# source zsh
function get_flake_config() {
  local hostname=$(hostname -s)
  local username=$(whoami)
  echo "${username}@${hostname}"
}

alias hms="home-manager switch --flake $NIX_CFG_PATH#$(get_flake_config)"

# system rebuild, host-agnostic: darwin on macos, nixos on linux
nrs() {
  local host="$(hostname -s)"
  case "$(uname -s)" in
    Darwin) sudo darwin-rebuild switch --flake "$NIX_CFG_PATH#$host" "$@" ;;
    Linux) sudo nixos-rebuild switch --flake "$NIX_CFG_PATH#$host" "$@" ;;
    *) echo "nrs: unsupported platform $(uname -s)" >&2; return 1 ;;
  esac
}

alias ll="ls -la"

alias d="dirs -v"
for index ({1..9}) alias "$index"="cd +${index}"; unset index

alias lg="lazygit"

# v:  daily driver, hms-installed nvim-live. lua edits are live (no hms needed);
#     only plugin/nix changes (categories.nix) require an hms.
# nv: nix run of the baked (wrapRc=true) package — use to test nix-file changes
#     before an hms, or to sanity-check the portable build.
# sessions are auto-managed in stdpath("state") when opened with no args
nv() { nix run $HOME/dots/modules/_nvim; }
v() { nvim; }
