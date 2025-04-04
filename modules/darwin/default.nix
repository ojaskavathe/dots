{
  lib,
  config,
  ...
}:
{
  imports = [
    ./aerospace.nix
    ./homebrew.nix
    ./kanata/kanata.nix
  ];

  aerospace = {
    enable = lib.mkDefault true;
    borders = lib.mkDefault true;
  };

  nix-hb = {
    enable = lib.mkDefault true;
  };

  kanata.enable = lib.mkDefault false;
}
