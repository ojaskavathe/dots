{ ... }:
{
  flake.modules.homeManager.codex =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      data = lib.importJSON ./codex-version.json;

      # Codex is packaged from the npm launcher `@openai/codex` rather than the
      # standalone release tarball. The tarball ships ONLY the `codex` binary; as
      # of 0.153 "code mode" is on by default and executes commands via a
      # companion `codex-code-mode-host` binary, failing closed if it's missing.
      # The npm package pulls a per-platform optionalDependency that vendors the
      # real binaries (codex, codex-code-mode-host, codex-app-server) side by
      # side, so buildNpmPackage installs the whole tree with every companion
      # present. The pinned package-lock.json + npmDepsHash keep it reproducible.
      codex-pkg = pkgs.buildNpmPackage {
        pname = "codex";
        inherit (data) version;
        nodejs = pkgs.nodejs_22;

        src = pkgs.fetchzip {
          url = "https://registry.npmjs.org/@openai/codex/-/codex-${data.version}.tgz";
          hash = data.srcHash;
        };

        npmDepsHash = data.npmDepsHash;
        makeCacheWritable = true;
        postPatch = ''cp ${./codex-package-lock.json} package-lock.json'';
        dontNpmBuild = true;

        installPhase = ''
          runHook preInstall
          mkdir -p $out/bin $out/lib/node_modules/@openai/codex
          cp -r . $out/lib/node_modules/@openai/codex/
          ln -s $out/lib/node_modules/@openai/codex/bin/codex.js $out/bin/codex
          chmod +x $out/bin/codex
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

      # Keys Nix manages in ~/.codex/config.toml. Codex writes the rest of this
      # file at runtime (auth, migrations, model NUX, [projects.*]), so instead
      # of owning the file we deep-merge these in on activation, leaving
      # everything codex wrote untouched.
      managedConfig = (pkgs.formats.toml { }).generate "codex-managed.toml" {
        # nix owns the binary, so codex's self-update check is pointless noise
        check_for_update_on_startup = false;
        # code mode is default-on since 0.153 and needs codex-code-mode-host,
        # which the npm packaging above now provides; assert it explicitly
        features.code_mode_host = true;
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

        # Layer the managed keys onto codex's own mutable config.toml (runtime
        # keys are preserved). Re-applied on every switch.
        home.activation.codexConfig = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
          mkdir -p $HOME/.codex
          CONFIG=$HOME/.codex/config.toml
          [ -f "$CONFIG" ] || : > "$CONFIG"
          ${pkgs.yq}/bin/tomlq -s -t '.[0] * .[1]' "$CONFIG" ${managedConfig} > "$CONFIG.merged"
          mv "$CONFIG.merged" "$CONFIG"
          chmod 600 "$CONFIG"
        '';
      };
    };
}
