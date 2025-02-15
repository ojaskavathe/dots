{
  lib,
  config,
  inputs,
  pkgs,
  username,
  ...
}:
{
  options = {
    nix-hb = {
      enable = lib.mkEnableOption "Enable Homebrew";
    };
  };

  config = lib.mkIf config.nix-hb.enable {
    nix-homebrew = {
      enable = true;
      enableRosetta = true; # arch -x86_64 brew

      user = username;

      taps = {
        "homebrew/homebrew-core" = inputs.homebrew-core;
        "homebrew/homebrew-cask" = inputs.homebrew-cask;
        "homebrew/homebrew-bundle" = inputs.homebrew-bundle;
      };

      # taps can no longer be added imperatively with `brew tap`.
      mutableTaps = false;
    };

    homebrew = {
      enable = true;
      onActivation.cleanup = "uninstall";

      # https://github.com/zhaofengli/nix-homebrew/issues/5#issuecomment-1878798641
      taps = builtins.attrNames config.nix-homebrew.taps;

      brews = [ "cowsay" ];
      casks = [];
    };
  };
}
