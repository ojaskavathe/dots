{
  config,
  lib,
  pkgs,
  pkgs-stable,
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
      # You can add overlays here
      overlays = [
        # If you want to use overlays exported from other flakes:
        # neovim-nightly-overlay.overlays.default

        # Or define it inline, for example:
        # (final: prev: {
        #   hi = final.hello.overrideAttrs (oldAttrs: {
        #     patches = [ ./change-hello-to-hi.patch ];
        #   });
        # })
      ];

      config = {
        allowUnfree = true;
        allowUnfreePredicate = _: true;
      };
    };

    home = {
      username = "dingus";
      homeDirectory = "/home/dingus";
      sessionVariables = {
        EDITOR = "nvim";
        BROWSER = "firefox";
        TERMINAL = "kitty";
        NIXOS_CONFIG = "$HOME/nixos-config";
        # NIXOS_CONFIG = builtins.toString config.my.configDir;
      };
    };

    # fonts.fontconfig.enable = true;
    home.packages = with pkgs; [
      # (nerdfonts.override { fonts = [ "JetBrainsMono" ]; })
      # nerd-fonts.jetbrains-mono
      ripgrep
      spotify
      mesa-demos
      brightnessctl
      qt6ct
      pavucontrol

      networkmanagerapplet

      adwaita-icon-theme

      # bluetooth frontend
      overskride

      steam

      tldr

      # audio patchbay
      qpwgraph

      multiviewer-for-f1
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
