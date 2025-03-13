{
  config,
  lib,
  pkgs,
  pkgs-stable,
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

  networking = {
    computerName = "camille";
    hostName = "camille.local";
    localHostName = "camille";
  };

  security.pam.enableSudoTouchIdAuth = true;

  # activationScripts are executed every time you boot the system or run `nixos-rebuild` / `darwin-rebuild`.
  system.activationScripts = {
    postUserActivation.text = ''
      # activateSettings -u will reload the settings from the database and apply them to the current session,
      # so we do not need to logout and login again to make the changes take effect.
      /System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings -u
    '';
  };

  # nixpkgs.overlays = [
  #   (self: super: {
  #     karabiner-elements = super.karabiner-elements.overrideAttrs (old: {
  #       version = "14.13.0";
  #
  #       src = super.fetchurl {
  #         inherit (old.src) url;
  #         hash = "sha256-gmJwoht/Tfm5qMecmq1N6PSAIfWOqsvuHU8VDJY8bLw=";
  #       };
  #     });
  #   })
  # ];
  # 
  # services.karabiner-elements = {
  #   enable = true;
  # };

  system.defaults = {
    menuExtraClock.Show24Hour = true;

    loginwindow = {
      GuestEnabled = false; # disable guest user
    };

    dock = {
      autohide = true;
      static-only = true;
      show-recents = false;
      tilesize = 32;

      # https://nikitabobko.github.io/AeroSpace/guide#a-note-on-mission-control
      expose-group-apps = true;
    };

    finder = {
      _FXShowPosixPathInTitle = true; # show full path in finder title
      AppleShowAllExtensions = true; # show all file extensions
      FXEnableExtensionChangeWarning = false; # disable warning when changing file extension
      QuitMenuItem = true; # enable quit menu item
      ShowPathbar = true; # show path bar
      ShowStatusBar = true; # show status bar
    };

    NSGlobalDomain = {
      # ctrl + cmd + mouse to drag windows
      NSWindowShouldDragOnGesture = true;

      ApplePressAndHoldEnabled = false;
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
