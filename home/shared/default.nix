{
  lib,
  config,
  ...
}:
{
  imports = [
    ./shell/zsh.nix
    ./git.nix
    ./shell/direnv.nix
    ./tmux.nix
    ./stylix-home.nix
    ./nvim/nvim.nix
    ./kitty/kitty.nix
    ./browser/zen.nix

    ./sops.nix

    ./claude-code.nix
  ];

  zsh.enable = lib.mkDefault true;
  direnv.enable = lib.mkDefault true;
  git.enable = lib.mkDefault true;
  tmux.enable = lib.mkDefault true;
  stylix-home.enable = lib.mkDefault true;

  nvim = {
    enable = lib.mkDefault true;
  };

  kitty.enable = lib.mkDefault false;

  zen.enable = lib.mkDefault false;

  claude-code.enable = lib.mkDefault false;
}
