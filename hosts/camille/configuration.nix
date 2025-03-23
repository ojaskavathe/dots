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

  # security.pam.enableSudoTouchIdAuth = true;
  security.pam.services.sudo_local.touchIdAuth = true;

  # activationScripts are executed every time you boot the system or run `nixos-rebuild` / `darwin-rebuild`.
  system.activationScripts = {
    postUserActivation.text = ''
      # activateSettings -u will reload the settings from the database and apply them to the current session,
      # so we do not need to logout and login again to make the changes take effect.
      /System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings -u
    '';
  };

  launchd = {
    daemons = {
      karabiner-driver = {
        command = ''
          /Library/Application Support/org.pqrs/Karabiner-DriverKit-VirtualHIDDevice/Applications/Karabiner-VirtualHIDDevice-Daemon.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Daemon
        '';
        serviceConfig = {
          KeepAlive = true;
          RunAtLoad = true;
        };
      };

      kanata = {
        command = ''
          /opt/homebrew/bin/kanata --cfg /Users/ojas/.config/kanata/hrm.kbd
        '';
        serviceConfig = {
          KeepAlive = true;
          RunAtLoad = true;
          StandardOutPath = "/tmp/kanata_daemon.out.log";
          StandardErrorPath = "/tmp/kanata_daemon.err.log";
        };
      };

      # kanata-tray = {
      #   command = "/Users/ojas/Downloads/kanata-tray-macos";
      #   environment = {
      #     KANATA_TRAY_LOG_DIR = "/tmp";
      #     HOME = "/Users/ojas";
      #     KANATA_TRAY_CONFIG_DIR = "/Users/ojas/.config/kanata-tray";
      #   };
      #   serviceConfig = {
      #     KeepAlive = true;
      #     RunAtLoad = true;
      #     StandardOutPath = "/tmp/kanata_tray_daemon.out.log";
      #     StandardErrorPath = "/tmp/kanata_tray_daemon.err.log";
      #   };
      # };
    };
  };

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
      AppleICUForce24HourTime = true;
      AppleInterfaceStyle = "Dark";
      AppleMeasurementUnits = "Centimeters";
      AppleMetricUnits = 1;

      "com.apple.keyboard.fnState" = false; # true -> fn keys are fn keys

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
          "61".enabled = false; # Ctrl + Alt + Space - Next Input Source
        };
      };
    };
  };

  aerospace = {
    enable = true;
  };
}
