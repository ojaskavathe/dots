# source zsh
function get_flake_config() {
  local hostname=$(hostname -s)
  local username=$(whoami)
  echo "${username}@${hostname}"
}

alias hms="home-manager switch --flake $NIX_CFG_PATH#$(get_flake_config)"
alias nrs="sudo nixos-rebuild switch --flake $NIX_CFG_PATH#$(hostname -s)"
alias drs="sudo darwin-rebuild switch --flake $NIX_CFG_PATH#$(hostname -s)"

alias ll="ls -la"

alias d="dirs -v"
for index ({1..9}) alias "$index"="cd +${index}"; unset index

alias lg="lazygit"

# nv: test nvim config changes without hms (nix run rebuilds if nix files changed)
# v:  daily driver, uses the hms-installed nvim
# sessions are auto-managed in stdpath("state") when opened with no args
nv() { nix run $NIX_CFG_PATH/home/shared/nvim; }
v() { nvim; }
