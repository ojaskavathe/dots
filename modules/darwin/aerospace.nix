{
  lib,
  config,
  inputs,
  pkgs,
  ...
}:
{
  options = {
    aerospace = {
      enable = lib.mkEnableOption "Enable Aerospace";
      borders = lib.mkEnableOption "Enable JankyBorders";
    };
  };

  config = lib.mkIf config.aerospace.enable {
    services.aerospace = {
      enable = true;
      settings = {
        default-root-container-layout = "tiles";
        automatically-unhide-macos-hidden-apps = true;

        gaps = {
          outer.left = 8;
          outer.bottom = 8;
          outer.top = 8;
          outer.right = 8;

          inner.horizontal = 8;
          inner.vertical = 8;
        };

        mode.main.binding = {
          alt-h = "focus left";
          alt-j = "focus down";
          alt-k = "focus up";
          alt-l = "focus right";

          alt-shift-h = "move left";
          alt-shift-j = "move down";
          alt-shift-k = "move up";
          alt-shift-l = "move right";

          alt-minus = "resize smart -50";
          alt-equal = "resize smart +50";

          alt-1 = "workspace 1";
          alt-2 = "workspace 2";
          alt-3 = "workspace 3";
          alt-4 = "workspace 4";
          alt-5 = "workspace 5";
          alt-6 = "workspace 6";
          alt-7 = "workspace 7";
          alt-8 = "workspace 8";
          alt-9 = "workspace 9";

          alt-shift-1 = "move-node-to-workspace 1";
          alt-shift-2 = "move-node-to-workspace 2";
          alt-shift-3 = "move-node-to-workspace 3";
          alt-shift-4 = "move-node-to-workspace 4";
          alt-shift-5 = "move-node-to-workspace 5";
          alt-shift-6 = "move-node-to-workspace 6";
          alt-shift-7 = "move-node-to-workspace 7";
          alt-shift-8 = "move-node-to-workspace 8";
          alt-shift-9 = "move-node-to-workspace 9";

          alt-f = "layout tiling floating"; # toggle floating

          alt-shift-semicolon = "mode service";
        };
        mode.service.binding = {
          f = [
            "layout floating tiling"
            "mode main"
          ]; # Toggle between floating and tiling layout
          backspace = [
            "close-all-windows-but-current"
            "mode main"
          ];
        };
      };
    };

    services.jankyborders = lib.mkIf config.aerospace.borders {
      enable = true;
      active_color = "0xffe1e3e4";
      inactive_color = "0xff494d63";
      width = 5.0;
    };
  };
}
