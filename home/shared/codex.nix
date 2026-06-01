{
  pkgs,
  lib,
  config,
  ...
}:
let
  data = lib.importJSON ./codex-version.json;
  platformSuffix =
    {
      aarch64-darwin = "aarch64-apple-darwin";
      x86_64-linux = "x86_64-unknown-linux-musl";
    }
    .${pkgs.stdenv.hostPlatform.system};

  codex-pkg = pkgs.stdenvNoCC.mkDerivation {
    pname = "codex";
    inherit (data) version;

    src = pkgs.fetchurl {
      url = "https://github.com/openai/codex/releases/download/rust-v${data.version}/codex-${platformSuffix}.tar.gz";
      hash = data.hashes.${pkgs.stdenv.hostPlatform.system};
    };

    sourceRoot = ".";
    dontBuild = true;
    dontStrip = true;

    installPhase = ''
      runHook preInstall
      install -Dm755 codex-${platformSuffix} $out/bin/codex
      runHook postInstall
    '';

    meta = {
      mainProgram = "codex";
      license = lib.licenses.asl20;
      platforms = [
        "aarch64-darwin"
        "x86_64-linux"
      ];
    };
  };
in
{

  options = {
    codex = {
      enable = lib.mkEnableOption "Enable OpenAI Codex CLI";
    };
  };

  config = lib.mkIf config.codex.enable {
    home.packages = [ codex-pkg ];
  };
}
