{
  config,
  lib,
  pkgs,
  pkgs-stable,
  username,
  ...
}:
{
  options = {
    my.configDir = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      apply = toString;
      default = ../.;
      description = "Location of the nix config directory (this repo)";
    };
  };
  config = {
    nixpkgs = {
      config = {
        allowUnfree = true;
        allowUnfreePredicate = _: true;
      };
    };
    home = {
      inherit username;
      homeDirectory = "/home/${username}";
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
