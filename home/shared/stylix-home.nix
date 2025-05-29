{
  pkgs,
  lib,
  config,
  ...
}:
{
  options = {
    stylix-home = {
      enable = lib.mkEnableOption "Enable Stylix";
    };
  };

  config = lib.mkIf config.stylix-home.enable {
    stylix = {
      enable = true;
      image = ../data/wallpapers/wp.jpg;
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
          terminal = 16;
        };
      };

      targets = {
        tmux.enable = false;
        neovim.enable = false;
      };
    };
  };
}
