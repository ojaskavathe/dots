{ lib, config, ... }:
{
  config = lib.mkIf config.hyprland.enable {
    wayland.windowManager.hyprland = {
      settings = {
        bind = [
          "$mainMod, RETURN, exec, $terminal tmux a || tmux"
          # "$mainMod, RETURN, exec, $terminal"
          "$mainMod, Q, killactive"
          "$mainMod, M, exit"
          "$mainMod, E, exec, $fileManager"
          "$mainMod, B, exec, $browser"
          "$mainMod, V, togglefloating"
          "$mainMod SHIFT, V, exec, [float] pavucontrol"
          "$mainMod SHIFT, S, exec, spotify"
          "$mainMod, F, fullscreen"
          "$mainMod, R, exec, $menu"
          "$mainMod, P, pseudo" # dwindle
          "$mainMod, T, togglesplit" # dwindle

          "$mainMod, Slash, exec, ags run-js 'cycleMode();'" # change bar

          # Move focus with mainMod + hjkl keys
          "$mainMod, H, movefocus, l"
          "$mainMod, L, movefocus, r"
          "$mainMod, K, movefocus, u"
          "$mainMod, J, movefocus, d"

          # Switch workspaces with mainMod + [0-9]
          "$mainMod, 1, workspace, 1"
          "$mainMod, 2, workspace, 2"
          "$mainMod, 3, workspace, 3"
          "$mainMod, 4, workspace, 4"
          "$mainMod, 5, workspace, 5"
          "$mainMod, 6, workspace, 6"
          "$mainMod, 7, workspace, 7"
          "$mainMod, 8, workspace, 8"
          "$mainMod, 9, workspace, 9"
          "$mainMod, 0, workspace, 10"

          # Move active window to a workspace with mainMod + SHIFT + [0-9]
          "$mainMod SHIFT, 1, movetoworkspace, 1"
          "$mainMod SHIFT, 2, movetoworkspace, 2"
          "$mainMod SHIFT, 3, movetoworkspace, 3"
          "$mainMod SHIFT, 4, movetoworkspace, 4"
          "$mainMod SHIFT, 5, movetoworkspace, 5"
          "$mainMod SHIFT, 6, movetoworkspace, 6"
          "$mainMod SHIFT, 7, movetoworkspace, 7"
          "$mainMod SHIFT, 8, movetoworkspace, 8"
          "$mainMod SHIFT, 9, movetoworkspace, 9"
          "$mainMod SHIFT, 0, movetoworkspace, 10"

          # Example special workspace (scratchpad)
          "$mainMod, S, togglespecialworkspace, magic"
          "$mainMod SHIFT, S, movetoworkspace, special:magic"

          # Scroll through existing workspaces with mainMod + scroll
          "$mainMod, mouse_down, workspace, e-1"
          "$mainMod, mouse_up, workspace, e+1"

          # Scroll through existing workspaces with mainMod + TAB
          "$mainMod , TAB, workspace, e+1"
          "$mainMod SHIFT, TAB, workspace, e-1"
        ];

        bindm = [
          # Move/resize windows with mainMod + LMB/RMB and dragging
          "$mainMod, mouse:272, movewindow"
          "$mainMod, mouse:273, resizewindow"
        ];

        binde = [
          # volume
          ",xf86audioraisevolume, exec, wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+ "
          ",xf86audiolowervolume, exec, wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%-"
          ",xf86audiomute, exec, wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"

          # mic
          ",xf86audiomicmute, exec, wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"

          # brightness
          ",XF86MonBrightnessUp,   exec, brightnessctl set +5%"
          ",XF86MonBrightnessDown, exec, brightnessctl set  5%-"
        ];
      };
    };
  };
}
