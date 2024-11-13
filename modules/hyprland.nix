{
  lib,
  config,
  inputs,
  pkgs,
  ...
}:
{
  options = {
    hyprland = {
      enable = lib.mkEnableOption "Enable Hyprland";
    };
  };

  config = lib.mkIf config.keyd.enable {
    nix.settings = {
      substituters = [ "https://hyprland.cachix.org" ];
      trusted-public-keys = [ "hyprland.cachix.org-1:a7pgxzMz7+chwVL3/pzj6jIBMioiJM7ypFP8PwtkuGc=" ];
    };
    programs.hyprland = {
      enable = true;
      # package = inputs.hyprland.packages.${pkgs.stdenv.hostPlatform.system}.hyprland; # set the flake package
      # portalPackage =
      #   inputs.hyprland.packages.${pkgs.stdenv.hostPlatform.system}.xdg-desktop-portal-hyprland; # keep portal in sync
    };
    environment.sessionVariables.NIXOS_OZONE_WL = "1";
  };
}
