{ pkgs, ... }:
{
  programs.ags = {
    enable = true;
    # configDir = ./ags;

    extraPackages = with pkgs; [
      gtksourceview
      gtksourceview4
      webkitgtk
    ];
  };
}
