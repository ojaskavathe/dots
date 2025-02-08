# source zsh
alias hm='home-manager switch --flake $NIXOS_CONFIG#dingus@tuf'
alias nrs='sudo nixos-rebuild switch --flake $NIXOS_CONFIG#tuf'

alias ll='ls -la'

alias d='dirs -v'
for index ({1..9}) alias "$index"="cd +${index}"; unset index
