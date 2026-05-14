{
  pkgs,
  lib,
  config,
  ...
}:
let
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
in
{

  options = {
    claude = {
      enable = lib.mkEnableOption "Enable Claude Code";
    };
  };

  config = lib.mkIf config.claude.enable {
    programs.claude-code = {
      enable = true;
      package = claude-code-pkg;
      settings = {
        model = "claude-opus-4-6";
        permissions = {
          defaultMode = "bypassPermissions";
        };
        skipDangerousModePermissionPrompt = true;
        preferences = {
          reasoning_effort = "high";
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
