{
  lib,
  config,
  ...
}:
{
  imports = [
    ./nvidia.nix
    ./keyd.nix
    ./hyprland.nix
  ];

  nvidia = {
    enable = lib.mkDefault false;
    optimus = lib.mkDefault false;
  };

  keyd = {
    enable = lib.mkDefault false;
  };

  hyprland = {
    enable = lib.mkDefault false;
  };
}
