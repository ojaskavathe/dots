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
    ./desktop/kde.nix
    ./desktop/hyprland/hyprland.nix

    ./kanata/kanata.nix
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

  kde.enable = lib.mkDefault false;
  hyprland.enable = lib.mkDefault false;

  kanata-macos.enable = lib.mkDefault false;
}
