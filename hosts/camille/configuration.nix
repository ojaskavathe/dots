{
  config,
  lib,
  pkgs,
  inputs,
  ...
}:

{
  environment = {
    systemPackages = with pkgs; [
    ];
  };

  nixpkgs.config = {
    allowUnfree = true;
  };
  nixpkgs.hostPlatform = "aarch64-darwin";

  nix.settings.experimental-features = [
    "nix-command"
    "flakes"
  ];

  system.stateVersion = 5;

  security.pam.enableSudoTouchIdAuth = true;

  system.defaults = {
    NSGlobalDomain = {
      # ctrl + cmd + mouse to drag windows  
      NSWindowShouldDragOnGesture = true;

      InitialKeyRepeat = 25;
      KeyRepeat = 2;
    };

    # https://github.com/LnL7/nix-darwin/issues/518
    # https://github.com/NUIKit/CGSInternal/blob/master/CGSHotKeys.h
    CustomUserPreferences = {
      "com.apple.symbolichotkeys" = {
        AppleSymbolicHotKeys = {
          "64".enabled = false; # Cmd + Space - Spotlight Search
          "65".enabled = false; # Cmd + Alt + Space - Finder Search Window
          "60".enabled = false; # Ctrl + Space - Previous Input Source
          "61".enabled = false; # Ctrol + Alt + Space - Next Input Source
        };
      };
    };
  };

  aerospace = {
    enable = true;
  };
}
