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
      homeDirectory = "/Users/${username}";
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
      tldr
      fastfetch
      ffmpeg
      lazygit
      yt-dlp

      monitorcontrol

      tailscale
      openvpn
      # syncthing

      slack
      google-chrome
      obsidian

      raycast

      dbeaver-bin

      nodejs
      beamMinimal27Packages.elixir-ls

      ollama

      # mods + .NET
      mono

      # latex
      texliveFull
      texlivePackages.latexmk
    ];

    nvim.enable = true;
    kitty.enable = true;

    stylix-home.enable = true;

    programs.fzf = {
      enable = true;
      enableZshIntegration = true;
    };

    programs.awscli = {
      enable = true;
      settings = {
        default = {
          region = "us-west-2";
          output = "json";
        };
      };
    };

    services.syncthing = {
      enable = true;
    };

    # Nicely reload system units when changing configs
    systemd.user.startServices = "sd-switch";

    # Enable home-manager and git
    programs.home-manager.enable = true;

    # https://nixos.wiki/wiki/FAQ/When_do_I_update_stateVersion
    home.stateVersion = "24.05";
  };
}
