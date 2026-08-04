# Home config for user `ojas` (camille / darwin). Sets which shared features are
# on and the user-specific packages. `pkgs-stable` rides in via the shared
# nixpkgs module; allowUnfree likewise, so it's no longer set here.
{ ... }:
{
  flake.modules.homeManager.ojas =
    {
      pkgs,
      lib,
      config,
      pkgs-stable,
      ...
    }:
    {
      home = {
        username = "ojas";
        homeDirectory = "/Users/ojas";
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

        # upstream pins sourceRoot = "Obsidian.app", but the dmg now nests the
        # app inside a volume folder. drop once NixOS/nixpkgs#548462 lands.
        # https://github.com/NixOS/nixpkgs/issues/548445
        (obsidian.overrideAttrs (old: {
          sourceRoot = "Obsidian ${old.version}-universal/Obsidian.app";
        }))

        dbeaver-bin

        nodejs

        ollama

        # mods + .NET
        mono

        sops

        # darwin.xcode
      ];

      programs.nvim = {
        enable = true;
        aliases = [
          "vim"
          "vi"
        ];
      };
      kitty.enable = true;

      stylix-home.enable = true;

      claude.enable = true;
      codex.enable = true;
      blender-mcp.enable = true;

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
