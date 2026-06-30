{
  config,
  lib,
  pkgs,
  pkgs-stable,
  username,
  inputs,
  system,
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
      gh
      ripgrep
      tldr
      fastfetch
      ffmpeg
      lazygit
      pkgs-stable.harlequin
      yt-dlp
      tree

      # macos specific
      monitorcontrol
      iina
      raycast
      unnaturalscrollwheels
      stats

      openvpn
      moonlight-qt
      # syncthing
      rclone
      localsend

      slack
      google-chrome
      obsidian

      dbeaver-bin

      nodejs
      beamMinimal28Packages.elixir-ls

      ollama

      # mods + .NET
      mono

      sops

      # darwin.xcode
    ];

    programs.nvim = {
      enable = true;
      # Dev variant (wrapRc = false): plugins/LSPs stay nix-managed, but the
      # lua config is read live from ~/.config/nvim (symlinked below), so
      # keybind/config edits need no rebuild — just relaunch nvim.
      package = inputs.nvim.packages.${system}.nvim-live;
      aliases = [
        "vim"
        "vi"
      ];
    };

    # Point ~/.config/nvim at the live repo (out-of-store symlink) so lua
    # edits apply immediately. Adding/removing plugins still needs an hms,
    # since that changes the nix-built nvim-live wrapper (categories.nix).
    xdg.configFile = {
      "nvim/init.lua".source =
        config.lib.file.mkOutOfStoreSymlink "${config.home.homeDirectory}/dots/home/shared/nvim/init.lua";
      "nvim/lua".source =
        config.lib.file.mkOutOfStoreSymlink "${config.home.homeDirectory}/dots/home/shared/nvim/lua";
    };
    kitty.enable = true;

    stylix-home.enable = true;

    claude.enable = true;
    codex.enable = true;

    programs.fzf = {
      enable = true;
      enableZshIntegration = false;
    };

    programs.zoxide = {
      enable = true;
    };

    programs.yazi = {
      enable = true;
      enableZshIntegration = true;
      shellWrapperName = "y";
    };

    programs.awscli = {
      enable = true;
      package = pkgs-stable.awscli2; # https://github.com/nixos/nixpkgs/issues/450617
      settings = {
        default = {
          region = "us-west-2";
          output = "json";
        };
      };
      credentials = {
        default = {
          credential_process = "${pkgs.writeShellScript "aws_credential_process.sh" ''
            ACCESS_KEY_FILE="${config.sops.secrets.aws_access_key_id.path}"
            SECRET_KEY_FILE="${config.sops.secrets.aws_secret_access_key.path}"

            ACCESS_KEY=$(cat "$ACCESS_KEY_FILE")
            SECRET_KEY=$(cat "$SECRET_KEY_FILE")

            echo '{
              "Version": 1,
              "AccessKeyId": "'$ACCESS_KEY'",
              "SecretAccessKey": "'$SECRET_KEY'"
            }'
          ''}";
        };
      };
    };

    zen.enable = false;

    services.syncthing = {
      enable = true;
    };

    sops-home.enable = true;

    # Nicely reload system units when changing configs
    systemd.user.startServices = "sd-switch";

    # Enable home-manager and git
    programs.home-manager.enable = true;

    # https://nixos.wiki/wiki/FAQ/When_do_I_update_stateVersion
    home.stateVersion = "24.05";
  };
}
