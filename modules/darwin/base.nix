# Base nix-darwin bundle: the modules + default enable states every darwin host
# gets. Mirrors the old modules/darwin/default.nix. Hosts override the defaults.
{ config, lib, ... }:
let
  dm = config.flake.modules.darwin;
in
{
  flake.modules.darwin.base = {
    imports = [
      dm.homebrew
      dm.kanata
    ];

    nix-hb = {
      enable = lib.mkDefault true;
    };

    kanata.enable = lib.mkDefault false;

    nix.gc = {
      automatic = lib.mkDefault true;
      options = lib.mkDefault "--delete-older-than 14d";
    };
  };
}
