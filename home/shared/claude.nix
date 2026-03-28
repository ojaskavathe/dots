{
  pkgs,
  lib,
  config,
  ...
}:
{

  options = {
    claude = {
      enable = lib.mkEnableOption "Enable Claude Code";
    };
  };

  config = lib.mkIf config.claude.enable {
    programs.claude-code = {
      enable = true;
      settings = {
        permissions = {
          defaultMode = "bypassPermissions";
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
