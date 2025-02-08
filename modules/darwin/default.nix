{
  lib,
  config,
  ...
}:
{
  imports = [
    ./aerospace.nix
  ];

  aerospace = {
    enable = lib.mkDefault true;
    borders = lib.mkDefault true;
  };
}
