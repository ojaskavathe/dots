{
  lib,
  config,
  ...
}:
{

  options = {
    kde = {
      enable = lib.mkEnableOption "Enable KDE";
    };
  };

  config = lib.mkIf config.kde.enable {
    # disable kwallet
    xdg.configFile = {
      "kwalletrc".text = ''
        [Wallet]
        Enabled=false
      '';
    };

    programs.plasma = {
      enable = true;
      # workspace = {
      #   lookAndFeel = "org.kde.breezedark.desktop";
      # };
      # fonts = {
      #   general = {
      #     family = "Sans Serif";
      #     pointSize = 10;
      #   };
      # };
      input = {
        mice = [
          {
            enable = true;
            name = "Logitech G304";
            vendorId = "046d";
            productId = "4074";
            acceleration = 0;
            accelerationProfile = "none";
            leftHanded = false;
            middleButtonEmulation = false;
            naturalScroll = false;
            scrollSpeed = 1;
          }
        ];
        touchpads = [
          {
            enable = true;
            name = "ELAN1203:00 04F3:307A Touchpad";
            vendorId = "04f3";
            productId = "307a";
            rightClickMethod = "twoFingers";
            scrollMethod = "twoFingers";
            disableWhileTyping = true;
            leftHanded = false;
            middleButtonEmulation = false;
            naturalScroll = true;
            pointerSpeed = 0;
            tapToClick = true;
          }
        ];
      };
      panels = [
        {
          location = "bottom";
          floating = true;
          height = 40;
          widgets = [
            {
              kickoff = {
                icon = "nix-snowflake-white";
                size = {
                  height = 250;
                  width = 250;
                };
              };
            }
            {
              iconTasks = {
                launchers = [
                  "applications:org.kde.dolphin.desktop"
                  "applications:kitty.desktop"
                  "applications:firefox.desktop"
                  "applications:spotify.desktop"
                ];
                size = {
                  height = 250;
                  width = 250;
                };
              };
            }
            "org.kde.plasma.marginsseparator"
            {
              systemTray.items = {
                # We explicitly show bluetooth and battery
                shown = [
                  "org.kde.plasma.clipboard"
                  "org.kde.plasma.volume"
                  "org.kde.plasma.battery"
                  "org.kde.plasma.bluetooth"
                  "org.kde.plasma.networkmanagement"
                ];
                hidden = [
                  "org.kde.plasma.brightness"
                ];
              };
            }
            {
              digitalClock = {
                date = {
                  format = "shortDate";
                  position = "belowTime";
                };
                calendar.firstDayOfWeek = "sunday";
                time = {
                  format = "24h";
                  showSeconds = "always";
                };
              };
            }
            "org.kde.plasma.showdesktop"
          ];
        }
      ];
    };
  };
}
