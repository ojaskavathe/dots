# Home config for user `dingus` on the TUF host (Linux desktop).
{ ... }:
{
  flake.modules.homeManager.dingus =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    {
      home = {
        username = "dingus";
        homeDirectory = "/home/dingus";
        sessionVariables = {
          EDITOR = "nvim";
          BROWSER = "firefox";
          TERMINAL = "kitty";
          NIX_CFG_PATH = "$HOME/dots";
        };
      };

      home.packages = with pkgs; [
        # essentials
        ripgrep
        brightnessctl
        tldr
        fastfetch

        # media
        spotify
        multiviewer-for-f1

        overskride # bluetooth frontend
        qpwgraph # audio patchbay
      ];

      stylix-home.enable = true;

      nvim.enable = true;
      kitty.enable = true;

      hyprland.enable = true;

      programs.chromium = {
        enable = true;
      };

      programs.fzf = {
        enable = true;
        enableZshIntegration = true;
      };

      # Nicely reload system units when changing configs
      systemd.user.startServices = "sd-switch";

      # Enable home-manager and git
      programs.home-manager.enable = true;

      # https://nixos.wiki/wiki/FAQ/When_do_I_update_stateVersion
      home.stateVersion = "24.05";
    };
}
