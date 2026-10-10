{ ... }:
{
  flake.modules.homeManager.claude =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      jsonFormat = pkgs.formats.json { };

      # home-manager's `programs.claude-code.mcpServers` doesn't write a plain MCP
      # config: it synthesises a plugin and injects it with --plugin-dir, which
      # namespaces everything as `plugin:claude-code-home-manager:<name>` and yields
      # tool names like `mcp__plugin_claude-code-home-manager_blender__*`. Wiring
      # --mcp-config ourselves keeps servers in the normal scope with clean
      # `mcp__<name>__*` tools, and is just as declarative (config lives in the store).
      #
      # The `=` in --mcp-config= is load-bearing: the flag is variadic, so the
      # space-separated form swallows following argv and `claude mcp list` dies with
      # "MCP config file not found: .../list".
      mcpConfig = jsonFormat.generate "claude-mcp-config.json" {
        inherit (config.claude) mcpServers;
      };
      hasMcpServers = config.claude.mcpServers != { };

      data = lib.importJSON ./claude-version.json;
      suffix =
        {
          aarch64-darwin = "darwin-arm64";
          x86_64-linux = "linux-x64";
        }
        .${pkgs.stdenv.hostPlatform.system};

      claude-code-pkg = pkgs.stdenvNoCC.mkDerivation {
        pname = "claude-code";
        inherit (data) version;

        src = pkgs.fetchurl {
          url = "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases/${data.version}/${suffix}/claude";
          hash = data.hashes.${pkgs.stdenv.hostPlatform.system};
        };

        dontUnpack = true;
        dontBuild = true;
        dontStrip = true;

        nativeBuildInputs = [
          pkgs.makeBinaryWrapper
        ]
        ++ lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.autoPatchelfHook ];

        installPhase = ''
          runHook preInstall
          install -Dm755 $src $out/bin/claude
          wrapProgram $out/bin/claude \
            --set DISABLE_AUTOUPDATER 1 \
            --set DISABLE_INSTALLATION_CHECKS 1 \
            --set USE_BUILTIN_RIPGREP 0 \
            ${lib.optionalString hasMcpServers "--add-flags '--mcp-config=${mcpConfig}'"} \
            --prefix PATH : ${
              lib.makeBinPath (
                [
                  pkgs.procps
                  pkgs.ripgrep
                ]
                ++ lib.optionals pkgs.stdenv.hostPlatform.isLinux [
                  pkgs.bubblewrap
                  pkgs.socat
                ]
              )
            }
          runHook postInstall
        '';

        meta = {
          mainProgram = "claude";
          license = lib.licenses.unfree;
          platforms = [
            "aarch64-darwin"
            "x86_64-linux"
          ];
        };
      };

      # the script itself is shared with windows (windows/setup.ps1 runs it under
      # Git Bash), so it calls its tools from PATH; pin them here
      statusline = pkgs.writeShellScript "claude-statusline" ''
        export PATH=${
          lib.makeBinPath [
            pkgs.jq
            pkgs.git
            pkgs.coreutils
            pkgs.gnused
          ]
        }:$PATH
        ${builtins.readFile ./claude-statusline.sh}
      '';

      # Keys Nix manages in ~/.claude/settings.json. Claude writes the rest of
      # this file at runtime (/config, plugins, permission rules), so instead of
      # owning the file we deep-merge these in on activation, like codex.nix.
      # The portable keys live in claude-settings.json, which windows/setup.ps1
      # merges the same way; only nix-specific ones are added here.
      managedSettings = jsonFormat.generate "claude-managed-settings.json" (
        lib.recursiveUpdate (lib.importJSON ./claude-settings.json) {
          # nix owns the binary, so the self-updater is dead weight
          env.DISABLE_AUTOUPDATER = "1";
          statusLine = {
            type = "command";
            command = "${statusline}";
          };
        }
      );
    in
    {

      options = {
        claude = {
          enable = lib.mkEnableOption "Enable Claude Code";

          mcpServers = lib.mkOption {
            type = lib.types.attrsOf jsonFormat.type;
            default = { };
            description = "MCP servers, wired via --mcp-config rather than home-manager's plugin mechanism";
          };
        };
      };

      config = lib.mkIf config.claude.enable {
        # package only: with `settings` empty, home-manager doesn't symlink a
        # read-only store file over ~/.claude/settings.json
        programs.claude-code = {
          enable = true;
          package = claude-code-pkg;
        };

        # Layer the managed keys onto claude's own mutable settings.json (runtime
        # keys are preserved). Re-applied on every switch. Runs after
        # linkGeneration so the stale store symlink from when home-manager owned
        # the file has already been cleaned up; a leftover one is dropped anyway.
        home.activation.claudeSettings = lib.hm.dag.entryAfter [ "linkGeneration" ] ''
          mkdir -p $HOME/.claude
          SETTINGS=$HOME/.claude/settings.json
          [ -L "$SETTINGS" ] && rm "$SETTINGS"
          [ -s "$SETTINGS" ] || echo '{}' > "$SETTINGS"
          ${pkgs.jq}/bin/jq -s '.[0] * .[1]' "$SETTINGS" ${managedSettings} > "$SETTINGS.merged"
          mv "$SETTINGS.merged" "$SETTINGS"
        '';

        home.packages = lib.mkIf config.sops-home.enable (
          with pkgs;
          [
            (writeShellScriptBin "claude-litellm" ''
              export ANTHROPIC_BASE_URL=$(cat "${config.sops.secrets.litellm_endpoint.path}")
              exec claude "$@"
            '')
          ]
        );
      };
    };
}
