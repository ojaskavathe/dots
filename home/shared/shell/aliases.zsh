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
# Both restore vim-obsession session if Session.vim exists in cwd
nv() { [ -f Session.vim ] && nix run $NIX_CFG_PATH/home/shared/nvim -- -S || nix run $NIX_CFG_PATH/home/shared/nvim; }
v() { [ -f Session.vim ] && nvim -S || nvim; }
