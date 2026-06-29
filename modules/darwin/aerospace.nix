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

  config = lib.mkIf config.aerospace.enable (
    let
      aerospaceBin = "${config.services.aerospace.package}/bin/aerospace";

      # One-shot: split workspaces 1..10 evenly across the connected monitors,
      # built-in display first. Invoked by the display watcher on each change.
      rebalanceWorkspaces = pkgs.writeShellScript "aerospace-rebalance-workspaces" ''
        set -eu
        export PATH=${pkgs.coreutils}/bin:$PATH

        aerospace=${lib.escapeShellArg aerospaceBin}
        jq=${lib.escapeShellArg "${pkgs.jq}/bin/jq"}
        workspace_count=10

        ids=$("$aerospace" list-monitors --json 2>/dev/null | "$jq" -c '
          [.[] | select(."monitor-name" | test("Built-in"; "i"))]
          + [.[] | select((."monitor-name" | test("Built-in"; "i")) | not)]
          | map(."monitor-id")
        ' 2>/dev/null || true)

        [ -n "$ids" ] || exit 0
        monitor_count=$(printf '%s' "$ids" | "$jq" 'length')
        [ "$monitor_count" -gt 0 ] || exit 0

        echo "$(date): rebalancing workspaces across monitors $ids"

        # Desired assignment as "workspace:monitor_id" pairs, built-in first.
        desired=""
        base=$((workspace_count / monitor_count))
        extra=$((workspace_count % monitor_count))
        workspace=1
        monitor_index=0
        while [ "$monitor_index" -lt "$monitor_count" ]; do
          monitor_id=$(printf '%s' "$ids" | "$jq" -r ".[$monitor_index]")
          group_size="$base"
          if [ "$monitor_index" -lt "$extra" ]; then
            group_size=$((group_size + 1))
          fi

          assigned=0
          while [ "$assigned" -lt "$group_size" ] && [ "$workspace" -le "$workspace_count" ]; do
            desired="$desired $workspace:$monitor_id"
            workspace=$((workspace + 1))
            assigned=$((assigned + 1))
          done

          monitor_index=$((monitor_index + 1))
        done

        # Snapshot live membership as "workspace:monitor_id" pairs, one query
        # per monitor (not per workspace).
        current_pairs() {
          for mid in $(printf '%s' "$ids" | "$jq" -r '.[]'); do
            for ws in $("$aerospace" list-workspaces --monitor "$mid" 2>/dev/null); do
              printf '%s:%s ' "$ws" "$mid"
            done
          done
        }

        # Move only the workspaces that aren't already on their target monitor.
        apply_assignment() {
          cur=" $(current_pairs) "
          for pair in $desired; do
            case "$cur" in
              *" $pair "*) ;;
              *)
                ws=$(printf '%s' "$pair" | cut -d: -f1)
                mon=$(printf '%s' "$pair" | cut -d: -f2)
                "$aerospace" move-workspace-to-monitor --workspace "$ws" "$mon" || true
                ;;
            esac
          done
        }

        # True when live membership already matches desired.
        assignment_matches() {
          cur=" $(current_pairs) "
          for pair in $desired; do
            case "$cur" in
              *" $pair "*) ;;
              *) return 1 ;;
            esac
          done
          return 0
        }

        # AeroSpace restores its own remembered monitor assignment when a
        # display reconnects, which races with and can clobber a single pass.
        # Park focus off the workspaces being moved, then re-apply until the
        # live layout matches desired and stays matched for two checks.
        focused=$("$aerospace" list-workspaces --focused 2>/dev/null || true)
        parked=0
        if [ "$monitor_count" -gt 1 ] && [ -n "$focused" ]; then
          temp=1
          while [ "$temp" -le "$workspace_count" ] && [ "$temp" = "$focused" ]; do
            temp=$((temp + 1))
          done
          "$aerospace" workspace "$temp" || true
          parked=1
        fi

        attempt=0
        ok_streak=0
        while [ "$attempt" -lt 8 ]; do
          apply_assignment
          sleep 0.5
          if assignment_matches; then
            ok_streak=$((ok_streak + 1))
            if [ "$ok_streak" -ge 2 ]; then
              break
            fi
          else
            ok_streak=0
          fi
          attempt=$((attempt + 1))
        done
        echo "$(date): settled (attempts=$attempt, ok_streak=$ok_streak)"

        if [ "$parked" -eq 1 ]; then
          "$aerospace" workspace "$focused" || true
        fi
      '';

      # Event-driven watcher: blocks on a CoreGraphics run loop and runs the
      # rebalance script only when displays are added/removed/reconfigured.
      displayWatcher = pkgs.stdenv.mkDerivation {
        pname = "aerospace-display-watcher";
        version = "1.0";
        dontUnpack = true;
        nativeBuildInputs = [ pkgs.swift ];
        buildPhase = ''
          swiftc -O -framework AppKit -framework CoreGraphics -framework Foundation \
            -o aerospace-display-watcher ${./aerospace-display-watcher.swift}
        '';
        installPhase = ''
          mkdir -p $out/bin
          cp aerospace-display-watcher $out/bin/
        '';
      };
    in
    {
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
          {
            # Bitwarden extension popup opens as its own browser window
            # titled "Bitwarden - ...". Float it across all browsers.
            "if".window-title-regex-substring = "Bitwarden";
            run = "layout floating";
          }
          # {
          #   "if".app-id = "app.zen-browser.zen";
          #   "if".app-name-regex-substring = "Picture-in-Picture";
          #   run = "layout floating";
          # }
        ];

        exec-on-workspace-change = [
          "/bin/bash"
          "${pkgs.writeShellScript "pip-move.sh" ''
            # Get current workspace
            current_workspace=$(aerospace list-workspaces --focused)

            # Move PiP windows to current workspace (handles both "Picture-in-Picture" and "Picture in Picture")
            aerospace list-windows --all | grep -E "(Picture-in-Picture|Picture in Picture)" | awk '{print $1}' | while read window_id; do
              if [ -n "$window_id" ]; then
                aerospace move-node-to-workspace --window-id "$window_id" "$current_workspace"
              fi
            done
          ''}"
        ];

        gaps = {
          outer.left = 0; # 8
          outer.bottom = 0; # 8
          outer.top = 0; # 8
          outer.right = 0; # 8

          inner.horizontal = 0; # 8
          inner.vertical = 0; # 8
        };

        mode.main.binding = {
          alt-h = "focus --boundaries all-monitors-outer-frame --boundaries-action stop left";
          alt-j = "focus --boundaries all-monitors-outer-frame --boundaries-action stop down";
          alt-k = "focus --boundaries all-monitors-outer-frame --boundaries-action stop up";
          alt-l = "focus --boundaries all-monitors-outer-frame --boundaries-action stop right";

          alt-shift-h = "move left";
          alt-shift-j = "move down";
          alt-shift-k = "move up";
          alt-shift-l = "move right";

          alt-ctrl-shift-h = "move-node-to-monitor --focus-follows-window left";
          alt-ctrl-shift-j = "move-node-to-monitor --focus-follows-window down";
          alt-ctrl-shift-k = "move-node-to-monitor --focus-follows-window up";
          alt-ctrl-shift-l = "move-node-to-monitor --focus-follows-window right";

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
          alt-t = "layout tiles";
          alt-a = "layout accordion";

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

    launchd.user.agents.aerospace-workspace-rebalance = {
      serviceConfig = {
        ProgramArguments = [
          "${displayWatcher}/bin/aerospace-display-watcher"
          (toString rebalanceWorkspaces)
        ];
        KeepAlive = true;
        RunAtLoad = true;
        StandardOutPath = "/tmp/aerospace_workspace_rebalance.out.log";
        StandardErrorPath = "/tmp/aerospace_workspace_rebalance.err.log";
      };
    };

    services.jankyborders = lib.mkIf config.aerospace.borders {
      enable = true;
      active_color = "0xffe1e3e4";
      inactive_color = "0xff494d63";
      width = 5.0;
    };

    system.defaults.spaces.spans-displays = false;
    }
  );
}
