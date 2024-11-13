{
  lib,
  config,
  ...
}:
{
  imports = [
    ./nvidia.nix
    ./kanata.nix
    ./hyprland.nix
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
}
