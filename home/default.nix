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
    ./nvim/nvim.nix
    ./kitty/kitty.nix
    ./desktop/kde.nix
    ./desktop/hyprland/hyprland.nix
  ];

  zsh.enable = lib.mkDefault true;
  direnv.enable = lib.mkDefault true;
  git.enable = lib.mkDefault true;
  tmux.enable = lib.mkDefault true;
  nvim = {
    enable = lib.mkDefault true;
  };

  kitty.enable = lib.mkDefault false;

  kde.enable = lib.mkDefault true;
  hyprland.enable = lib.mkDefault false;
}
