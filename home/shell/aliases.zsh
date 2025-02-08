# source zsh
function get_flake_config() {
  local hostname=$(hostname -s)
  local username=$(whoami)
  echo "${username}@${hostname}"
}

alias hms="home-manager switch --flake $NIX_CFG_PATH#$(get_flake_config)"
alias nrs="sudo nixos-rebuild switch --flake $NIX_CFG_PATH#$(whoami)"
alias drs="darwin-rebuild switch --flake $NIX_CFG_PATH#$(whoami)"

alias ll="ls -la"

alias d="dirs -v"
for index ({1..9}) alias "$index"="cd +${index}"; unset index
