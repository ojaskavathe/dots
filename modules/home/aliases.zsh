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

# v:  daily driver, the hms-installed nvim. config is baked into the nix build,
#     so lua/plugin edits need an hms to take effect.
# nv: nix run of the same package straight from the repo — handy to test changes
#     before an hms, or to sanity-check the portable build on another machine.
# sessions are auto-managed in stdpath("state") when opened with no args
nv() { nix run $HOME/dots/modules/_nvim; }
v() { nvim; }
