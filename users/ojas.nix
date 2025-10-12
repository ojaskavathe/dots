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
      tree

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

      sops

      # darwin.xcode
    ];

    nvim.enable = true;
    kitty.enable = true;

    stylix-home.enable = true;

    claude-code.enable = true;

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

    # Nicely reload system units when changing configs
    systemd.user.startServices = "sd-switch";

    # Enable home-manager and git
    programs.home-manager.enable = true;

    # https://nixos.wiki/wiki/FAQ/When_do_I_update_stateVersion
    home.stateVersion = "24.05";
  };
}
