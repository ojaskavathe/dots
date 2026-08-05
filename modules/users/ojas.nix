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
    let
      # Put real file *references* on the macOS clipboard (not just path text),
      # so ⌘V in Finder pastes the files themselves. pbcopy can't do this;
      # NSPasteboard via JXA can. Handles multiple selected files.
      yaziClip = pkgs.writeShellScript "yazi-clip" ''
        osascript -l JavaScript -e '
        ObjC.import("AppKit");
        function run(argv) {
          const pb = $.NSPasteboard.generalPasteboard;
          pb.clearContents;
          const arr = $.NSMutableArray.alloc.init;
          argv.forEach(p => arr.addObject($.NSURL.fileURLWithPath(p)));
          pb.writeObjects(arr);
        }' "$@"
      '';
    in
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
      grok.enable = true;
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

        # yank (y): copy the selected/hovered files to the macOS clipboard as
        # real file references (⌘V pastes the files in Finder), then yank inside
        # yazi as usual (so `p` still pastes within yazi).
        keymap.mgr.prepend_keymap = [
          {
            on = "y";
            run = [
              "shell -- ${yaziClip} %s"
              "yank"
            ];
            desc = "Copy files to clipboard, then yank";
          }
        ];

        # show a symlink's target in the status bar
        initLua = ''
          Status:children_add(function(self)
            local h = self._current.hovered
            if h and h.link_to then
              return " -> " .. tostring(h.link_to)
            else
              return ""
            end
          end, 3300, Status.LEFT)
        '';
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
