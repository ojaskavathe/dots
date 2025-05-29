{
  pkgs,
  lib,
  config,
  ...
}:
{

  options = {
    hyprland = {
      enable = lib.mkEnableOption "Enable hyprland";
    };
  };

  imports = [
    ./binds.nix
  ];

  config = lib.mkIf config.hyprland.enable {
    xdg.configFile = {
      "hypr/scripts/bitwarden-float.sh".source = (import ./scripts/bitwarden-float.nix pkgs);
    };

    wayland.windowManager.hyprland = {
      enable = true;

      settings = {
        "$mainMod" = "SUPER";
        "$terminal" = "kitty";
        "$fileManager" = "dolphin";
        "$menu" = "wofi --show drun";
        "$browser" = "firefox";

        exec-once = [
          "$XDG_CONFIG_HOME/hypr/scripts/bitwarden-float.sh"
          "lxqt-policykit-agent"
        ];

        windowrule = [
          "float,lxqt-policykit-agent"
          "center,lxqt-policykit-agent"
          "size 500 500,lxqt-policykit-agent"

          "float,nm-connection-editor"
        ];

        monitor = [
          "eDP-1, 1920x1080@144, 0x0, 1.25"
        ];

        input = {
          follow_mouse = 1;
          accel_profile = "flat";
          sensitivity = 0; # -1.0 -> 1.0, 0 means no modification
          touchpad = {
            natural_scroll = "yes";
          };
        };

        env = [
          "XCURSOR_SIZE, 24"
          "QT_QPA_PLATFORM,wayland"

          # Select Integrated AMD GPU > Nvidia (priority)
          # "AQ_DRM_DEVICES,/dev/dri/card2:/dev/dri/card1"

          # toolkit-specific scale
          "GDK_SCALE,1"
          # "GTK_THEME,Breeze"
        ];

        # xwayland = {
        #   force_zero_scaling = true;
        # };

        decoration = {
          rounding = 2;
        };
      };
    };

    home.packages = with pkgs; [
      pavucontrol
      kdePackages.qtsvg
    ];

  #   programs.fuzzel = {
  #     enable = true;
  #     settings = {
  #       main = {
  #         font = "Gabarito";
  #         terminal = "${pkgs.kitty}/bin/kitty";
  #         prompt = "_> ";
  #         layer = "overlay";
  #       };
  #
  #       colors = {
  #         background = "000000ff";
  #         text = "e2e2e2ff";
  #         selection = "242424ff";
  #         selection-text = "e2e2e2ff";
  #         border = "242424ff";
  #         match = "e2e2e2ff";
  #         selection-match = "e2e2e2ff";
  #       };
  #
  #       border = {
  #         radius = 17;
  #         width = 2;
  #       };
  #
  #       dmenu.exit-immediately-if-empty = "yes";
  #     };
  #   };
  };
}
