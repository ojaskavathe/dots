{ ... }:
{
  flake.modules.homeManager.kitty =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    {
      options = {
        kitty = {
          enable = lib.mkEnableOption "Enable Kitty";
        };
      };

      config = lib.mkIf config.kitty.enable {
        # nixpkgs = {
        #   overlays = [
        #     # change icon
        #     (final: prev: {
        #       kitty = prev.kitty.overrideAttrs (oldAttrs: {
        #         postInstall =
        #           (oldAttrs.postInstall or "")
        #           + ''
        #             cp -f ${./kitty.app.png} $out/share/icons/hicolor/256x256/apps/kitty.png
        #             rm -f $out/share/icons/hicolor/scalable/apps/kitty.svg
        #           '';
        #       });
        #     })
        #
        #   ];
        # };

        programs.kitty = {
          enable = true;
          settings = {
            confirm_os_window_close = 0;
            # hide_window_decorations = "titlebar-only";
          };
          keybindings = {
            # https://github.com/kovidgoyal/kitty/issues/948#issuecomment-1107216743
            "cmd+h" = "no_op";
            "cmd+l" = "no_op";
            "cmd+t" = "no_op";
            # macOS treats opt as a compose key, so opt+a types "å" and the
            # tmux `bind -n M-a` never fires. Rather than macos_option_as_alt,
            # which costs every accented character, each opt chord tmux binds
            # is forwarded explicitly. Adding a `bind -n M-<key>` in tmux.nix
            # means adding it HERE too, or the bind is silently dead.
            "opt+s" = "send_key alt+s"; # winch: toggle the sidebar
            "opt+a" = "send_key alt+a"; # winch: the agent switcher
            "opt+e" = "send_key alt+e"; # tmux-equalize-nvim
            "opt+g" = "send_key alt+g"; # clear screen + scrollback
          };
        };
      };
    };
}
