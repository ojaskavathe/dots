{
  lib,
  config,
  ...
}:
{
  imports = [
    ./nvidia.nix
    ./keyd.nix
  ];

  nvidia = {
    enable = lib.mkDefault false;
    optimus = lib.mkDefault false;
  };

  keyd = {
    enable = lib.mkDefault false;
  };
}
