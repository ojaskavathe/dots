# source zsh
alias hm='home-manager switch --flake ~/nixos-config#dingus@nixos'
alias nrs='sudo nixos-rebuild switch --flake ~/nixos-config#nixos'

alias gs='git status'
alias ga='git add'
alias gp='git push'
alias gpo='git push origin'
alias gtd='git tag --delete'
alias gtdr='git tag --delete origin'
alias gr='git branch -r'
alias gplo='git pull origin'
alias gb='git branch '
alias gc='git commit'
alias gd='git diff'
alias gco='git checkout '
alias gl='git log'
alias gr='git remote'
alias grs='git remote show'
alias glo='git log --pretty="oneline"'
alias glol='git log --graph --oneline --decorate'

alias d='dirs -v'
for index ({1..9}) alias "$index"="cd +${index}"; unset index
