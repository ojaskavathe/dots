# Linux-desktop home bundle (KDE / Hyprland). Replaces home/nixos/default.nix.
# Only used by Linux home configs; darwin never imports it.
{ config, lib, ... }:
let
  hm = config.flake.modules.homeManager;
in
{
  flake.modules.homeManager.linux = {
    imports = [
      hm.kde
      hm.hyprland
    ];

    kde.enable = lib.mkDefault false;
    hyprland.enable = lib.mkDefault false;
  };
}
