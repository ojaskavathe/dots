# source zsh
alias hm='home-manager switch --flake $NIXOS_CONFIG#dingus@nixos'
alias nrs='sudo nixos-rebuild switch --flake $NIXOS_CONFIG#nixos'

alias d='dirs -v'
for index ({1..9}) alias "$index"="cd +${index}"; unset index
