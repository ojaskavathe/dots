{ ... }:
{
  flake.modules.homeManager.grok =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      data = lib.importJSON ./grok-version.json;
      # xAI names artifacts <os>-<arch>; nixpkgs uses <arch>-<os>.
      platformSuffix =
        {
          aarch64-darwin = "macos-aarch64";
          x86_64-linux = "linux-x86_64";
        }
        .${pkgs.stdenv.hostPlatform.system};

      grok-pkg = pkgs.stdenvNoCC.mkDerivation {
        pname = "grok";
        inherit (data) version;

        # GCS artifact bucket rather than the x.ai/cli Cloudflare front, so the
        # fetch hash stays stable and isn't subject to edge redirects.
        src = pkgs.fetchurl {
          url = "https://storage.googleapis.com/grok-build-public-artifacts/cli/grok-${data.version}-${platformSuffix}";
          hash = data.hashes.${pkgs.stdenv.hostPlatform.system};
        };

        dontUnpack = true;
        dontBuild = true;
        dontStrip = true;

        nativeBuildInputs = lib.optionals pkgs.stdenv.hostPlatform.isLinux [
          pkgs.autoPatchelfHook
        ];
        buildInputs = lib.optionals pkgs.stdenv.hostPlatform.isLinux [
          pkgs.stdenv.cc.cc.lib
        ];

        # Upstream ships one binary and symlinks both `grok` and `agent` to it.
        installPhase = ''
          runHook preInstall
          install -Dm755 $src $out/bin/grok
          ln -s grok $out/bin/agent
          runHook postInstall
        '';

        meta = {
          description = "Grok Build — xAI's terminal coding agent";
          homepage = "https://x.ai/cli";
          mainProgram = "grok";
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
        grok = {
          enable = lib.mkEnableOption "Enable Grok Build CLI";
        };
      };

      config = lib.mkIf config.grok.enable {
        home.packages = [ grok-pkg ];
      };
    };
}
