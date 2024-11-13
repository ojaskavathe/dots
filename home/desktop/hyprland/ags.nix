{
  pkgs,
  config,
  lib,
  ...
}:
{
  config = lib.mkIf config.hyprland.enable {
    programs.ags = {
      enable = true;
      configDir = ./ags;

      extraPackages = with pkgs; [
        gtksourceview
        webkitgtk
        accountsservice
      ];
    };
  };
}
