# Home config for user `dingus` on the WSL host (galio-wsl).
{ ... }:
{
  flake.modules.homeManager.dingus-wsl =
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
          TERMINAL = "wsl";
          NIX_CFG_PATH = "$HOME/dots";
        };
      };
      home.packages = with pkgs; [
        # essentials
        gh
        ripgrep
        tldr
        fastfetch
        ffmpeg
        lazygit
        yt-dlp
        tree
        sops
        claude-code
      ];
      programs.nvim = {
        enable = true;
        aliases = [
          "vim"
          "vi"
        ];
      };
      claude.enable = true;
      programs.fzf = {
        enable = true;
        enableZshIntegration = true;
      };
      programs.zoxide = {
        enable = true;
      };
      programs.yazi = {
        enable = true;
        enableZshIntegration = true;
        shellWrapperName = "y";
      };
      sops-home.enable = true;
      stylix-home.enable = false;
      # Nicely reload system units when changing configs
      systemd.user.startServices = "sd-switch";
      programs.home-manager.enable = true;
      home.stateVersion = "25.05";
    };
}
