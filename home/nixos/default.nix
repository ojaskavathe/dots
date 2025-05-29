{
  lib,
  config,
  ...
}:
{
  imports = [
    ./desktop/kde.nix
    ./desktop/hyprland/hyprland.nix
  ];

  kde.enable = lib.mkDefault false;
  hyprland.enable = lib.mkDefault false;
}
