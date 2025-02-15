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

        on-window-detected = [
          {
            "if".app-id = "com.apple.finder";
            run = "layout floating";
          }
          {
            "if".app-id = "com.apple.Preview";
            run = "layout floating";
          }
          {
            "if".app-name-regex-substring = "quicktime";
            run = "layout floating";
          }
        ];

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
          alt-0 = "workspace 10";

          alt-shift-1 = "move-node-to-workspace 1 --focus-follows-window";
          alt-shift-2 = "move-node-to-workspace 2 --focus-follows-window";
          alt-shift-3 = "move-node-to-workspace 3 --focus-follows-window";
          alt-shift-4 = "move-node-to-workspace 4 --focus-follows-window";
          alt-shift-5 = "move-node-to-workspace 5 --focus-follows-window";
          alt-shift-6 = "move-node-to-workspace 6 --focus-follows-window";
          alt-shift-7 = "move-node-to-workspace 7 --focus-follows-window";
          alt-shift-8 = "move-node-to-workspace 8 --focus-follows-window";
          alt-shift-9 = "move-node-to-workspace 9 --focus-follows-window";
          alt-shift-0 = "move-node-to-workspace 10 --focus-follows-window";

          alt-ctrl-h = "focus-monitor left";
          alt-ctrl-j = "focus-monitor down";
          alt-ctrl-k = "focus-monitor up";
          alt-ctrl-l = "focus-monitor right";

          alt-ctrl-1 = "summon-workspace 1";
          alt-ctrl-2 = "summon-workspace 2";
          alt-ctrl-3 = "summon-workspace 3";
          alt-ctrl-4 = "summon-workspace 4";
          alt-ctrl-5 = "summon-workspace 5";
          alt-ctrl-6 = "summon-workspace 6";
          alt-ctrl-7 = "summon-workspace 7";
          alt-ctrl-8 = "summon-workspace 8";
          alt-ctrl-9 = "summon-workspace 9";
          alt-ctrl-0 = "summon-workspace 10";

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
