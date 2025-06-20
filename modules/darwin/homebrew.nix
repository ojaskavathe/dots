{
  lib,
  config,
  inputs,
  pkgs,
  primaryUser,
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

      user = primaryUser;

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
      onActivation.cleanup = "zap";
      onActivation.autoUpdate = true;
      onActivation.upgrade = true;

      # https://github.com/zhaofengli/nix-homebrew/issues/5#issuecomment-1878798641
      taps = builtins.attrNames config.nix-homebrew.taps;

      brews = [
        "cowsay"
        "kanata"
        "mas" # cli for mas app ids
      ];
      casks = [
        "spotify"
        "cursor"
        "discord"
        "steam"
        "epic-games"
        "msty"
        "multiviewer-for-f1"
        "clocker"
        "whisky"
        "crossover"
        "kicad"
        "obs"
        "parsec"
        "trex"
      ];

      masApps = {
        "Amphetamine" = 937984704; 
      };
    };
  };
}
