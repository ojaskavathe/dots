{
  lib,
  config,
  ...
}:
{
  imports = [
    ./nvidia.nix
  ];

  nvidia = {
    enable = lib.mkDefault false;
    optimus = lib.mkDefault false;
  };
}
