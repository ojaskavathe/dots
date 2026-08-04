# Base NixOS bundle: the modules + default enable states shared across NixOS
# hosts. Mirrors the old modules/nixos/default.nix. Hosts override the defaults.
{ config, lib, ... }:
let
  nm = config.flake.modules.nixos;
in
{
  flake.modules.nixos.base = {
    imports = [
      nm.nvidia
      nm.kanata
      nm.hyprland
    ];

    nvidia = {
      enable = lib.mkDefault false;
      optimus = lib.mkDefault false;
    };

    kanata = {
      enable = lib.mkDefault false;
    };

    hyprland = {
      enable = lib.mkDefault false;
    };
  };
}
