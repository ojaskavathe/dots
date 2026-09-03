{ inputs, ... }:
{
  flake.modules.homeManager.stylix =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    {
      imports = [ inputs.stylix.homeModules.stylix ];

      options = {
        stylix-home = {
          enable = lib.mkEnableOption "Enable Stylix";
        };
      };

      config = lib.mkIf config.stylix-home.enable {
        # stylix's x11 target sets home.pointerCursor.x11.enable=true
        # unconditionally but only provides the cursor name/package on Linux
        # (hm/cursor.nix is isLinux-gated). The newer home-manager then treats
        # pointerCursor as enabled and its cursor/xdg blocks demand a cursor
        # name macOS has no source for -> eval error. Cursors are meaningless
        # on darwin, so force the whole module off there.
        home.pointerCursor.enable = lib.mkIf pkgs.stdenv.hostPlatform.isDarwin (lib.mkForce false);

        stylix = {
          enable = true;
          image = inputs.self + /data/wallpapers/wp.jpg;
          base16Scheme = "${pkgs.base16-schemes}/share/themes/catppuccin-mocha.yaml";
          polarity = "dark";

          cursor = {
            package = pkgs.bibata-cursors;
            name = "Bibata-Modern-Ice";
            size = 22;
          };

          fonts = {
            monospace = {
              package = pkgs.nerd-fonts.jetbrains-mono;
              name = "JetBrainsMonoNL Nerd Font Mono";
            };
            sansSerif = {
              package = pkgs.rubik;
              name = "Rubik";
            };

            sizes = {
              terminal = 12;
            };
          };

          targets = {
            tmux.enable = false;
            neovim.enable = false;
            # GTK theming is meaningless on macOS (no GTK apps), and stylix's gtk
            # target now sets gtk.gtk4.theme unconditionally, which conflicts.
            gtk.enable = !pkgs.stdenv.hostPlatform.isDarwin;
          };
        };
      };
    };
}
