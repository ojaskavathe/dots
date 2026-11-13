{ ... }:
{
  flake.modules.homeManager.helium =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      data = lib.importJSON ./helium-version.json;

      # Helium (imputnet's Chromium) isn't in nixpkgs: the official, notarized
      # build from its GitHub releases, pinned here and bumped by `just update-helium`.
      # Its own updater can't write into the store, so updates come only from here.
      helium-pkg = pkgs.stdenvNoCC.mkDerivation {
        pname = "helium";
        inherit (data) version;

        src = pkgs.fetchurl {
          url = "https://github.com/imputnet/helium-macos/releases/download/${data.version}/helium_${data.version}_arm64-macos.dmg";
          inherit (data) hash;
        };

        # An HFS+ image, LZMA-compressed: 7-Zip opens it without mounting anything.
        nativeBuildInputs = [ pkgs._7zz ];
        unpackPhase = ''
          runHook preUnpack
          7zz x -snld -y "$src" >/dev/null
          runHook postUnpack
        '';

        dontBuild = true;
        dontFixup = true; # signed by imput: patching or stripping would break the signature

        installPhase = ''
          runHook preInstall
          app=$(find . -maxdepth 2 -name Helium.app -type d | head -1)
          mkdir -p $out/Applications
          cp -R "$app" $out/Applications/
          runHook postInstall
        '';

        meta = {
          description = "Helium, a private, fast and honest Chromium";
          homepage = "https://helium.computer";
          license = lib.licenses.gpl3Only;
          platforms = [ "aarch64-darwin" ];
          sourceProvenance = [ lib.sourceTypes.binaryNativeCode ];
        };
      };
    in
    {
      options = {
        helium = {
          enable = lib.mkEnableOption "Enable Helium, the browser";
        };
      };

      config = lib.mkIf config.helium.enable {
        home.packages = [ helium-pkg ];
      };
    };
}
