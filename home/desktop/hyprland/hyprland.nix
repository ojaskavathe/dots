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

    wayland.windowManager.hyprland = {
      enable = true;

      settings = {
        "$mainMod" = "SUPER";
        "$terminal" = "kitty";
        "$fileManager" = "dolphin";
        "$menu" = "wofi --show drun";
        "$browser" = "firefox";

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
          "QT_QPA_PLATFORMTHEME,kde" # change to qt6ct if you have that

          # Select Integrated AMD GPU > Nvidia (priority)
          # "AQ_DRM_DEVICES,/dev/dri/card2:/dev/dri/card1"

          # toolkit-specific scale
          "GDK_SCALE,2"
          "GTK_THEME,Adwaita:dark"
        ];

        xwayland = {
          force_zero_scaling = true;
        };
      };
    };
  };
}
