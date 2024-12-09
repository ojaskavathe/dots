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
      };

      fonts = {
        monospace = {
          # package = pkgs.nerd-fonts.jetbrains-mono;
          # name = "JetBrainsMono Nerd Font Mono";
          package = pkgs.nerd-fonts.caskaydia-cove;
          name = "CaskaydiaCove Nerd Font Mono";
        };
        sansSerif = {
          package = pkgs.rubik;
          name = "Rubik";
        };
      };

      targets = {
        tmux.enable = false;
        neovim.enable = false;
      };
    };
  };
}
