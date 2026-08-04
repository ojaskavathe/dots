# Base nix-darwin bundle: the modules + default enable states every darwin host
# gets. Mirrors the old modules/darwin/default.nix. Hosts override the defaults.
{ config, lib, ... }:
let
  dm = config.flake.modules.darwin;
in
{
  flake.modules.darwin.base = {
    imports = [
      dm.aerospace
      dm.homebrew
      dm.kanata
    ];

    aerospace = {
      enable = lib.mkDefault true;
      borders = lib.mkDefault true;
    };

    nix-hb = {
      enable = lib.mkDefault true;
    };

    kanata.enable = lib.mkDefault false;
  };
}
