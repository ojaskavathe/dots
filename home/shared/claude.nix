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

  statusline = pkgs.writeShellScript "claude-statusline" ''
    input=$(cat)
    jq() { ${pkgs.jq}/bin/jq "$@"; }

    model=$(printf '%s' "$input" | jq -r '.model.display_name')
    dir=$(printf '%s' "$input" | jq -r '.workspace.current_dir' | sed "s|^$HOME|~|")
    pct=$(printf '%s' "$input" | jq -r '.context_window.used_percentage // 0 | floor')
    used=$(printf '%s' "$input" | jq -r '.context_window.total_input_tokens // 0')
    max=$(printf '%s' "$input" | jq -r '.context_window.context_window_size // 0')

    numfmt() { LC_ALL=en_US.UTF-8 ${pkgs.coreutils}/bin/numfmt --to=si "$@"; }
    usedfmt=$(printf '%s' "$used" | numfmt 2>/dev/null || printf '%s' "$used")
    maxfmt=$(printf '%s' "$max" | numfmt 2>/dev/null || printf '%s' "$max")

    printf '%s · %s · ctx %s%% (%s/%s)' "$model" "$dir" "$pct" "$usedfmt" "$maxfmt"
  '';
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
    programs.claude-code = {
      enable = true;
      package = claude-code-pkg;
      settings = {
        tui = "fullscreen";
        theme = "auto";
        permissions = {
          defaultMode = "bypassPermissions";
        };
        skipDangerousModePermissionPrompt = true;
        preferences = {
          reasoning_effort = "high";
        };
        statusLine = {
          type = "command";
          command = "${statusline}";
        };
      };
    };

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
}
