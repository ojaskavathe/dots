# `ok`: one entry point for machine commands. Each command is a script registered in
# `ok.commands`; downstream flakes add their own by setting more attrs.
{ ... }:
{
  flake.modules.homeManager.ok =
    {
      config,
      lib,
      pkgs,
      ...
    }:
    let
      cfg = config.ok;

      commandDrv =
        name: command:
        pkgs.writeShellApplication {
          name = "ok-${name}";
          inherit (command) runtimeInputs;
          text = builtins.readFile command.script;
        };

      commandsDir = pkgs.linkFarm "ok-commands" (
        lib.mapAttrsToList (name: command: {
          inherit name;
          path = lib.getExe (commandDrv name command);
        }) cfg.commands
      );

      ok = pkgs.writeShellApplication {
        name = "ok";
        runtimeInputs = [
          pkgs.coreutils
          pkgs.gnused
        ];
        runtimeEnv.OK_COMMANDS = commandsDir;
        text = builtins.readFile ./ok/ok;
      };
    in
    {
      options.ok = {
        enable = lib.mkEnableOption "the ok command dispatcher";

        commands = lib.mkOption {
          description = ''
            Commands available as `ok <name>`. The script's first `# ok: ` line is its
            one-line description; an optional `# ok-actions: a b` line feeds completion.
          '';
          default = { };
          type = lib.types.attrsOf (
            lib.types.submodule {
              options = {
                script = lib.mkOption { type = lib.types.path; };
                runtimeInputs = lib.mkOption {
                  type = lib.types.listOf lib.types.package;
                  default = [ ];
                };
              };
            }
          );
        };
      };

      config = lib.mkIf cfg.enable {
        ok.commands = {
          hms.script = ./ok/hms;
          nrs.script = ./ok/nrs;
          prebuild.script = ./ok/prebuild;
        };

        home.packages = [ ok ];

        programs.zsh = {
          shellAliases = {
            hms = "ok hms";
            nrs = "ok nrs";
          };
          initContent = lib.mkOrder 1100 ''
            _ok() {
              if (( CURRENT == 2 )); then
                compadd -- help which $(ok --list)
              elif (( CURRENT == 3 )); then
                compadd -- $(ok --actions "''${words[2]}")
              else
                _files
              fi
            }
            compdef _ok ok
          '';
        };
      };
    };
}
