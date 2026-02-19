{
  pkgs,
  lib,
  config,
  ...
}:
{

  options = {
    claude-code = {
      enable = lib.mkEnableOption "Enable Claude Code";
    };
  };

  config = lib.mkIf config.claude-code.enable {
    programs.claude-code = {
      enable = true;
      settings = {
        # apiKeyHelper = "${pkgs.writeShellScript "generate_temp_api_key.sh" ''
        #   cat "${config.sops.secrets.litellm_api_key.path}"
        # ''}";

        permissions = {
          allow = [
            "Bash(git diff:*)"
            "Edit"
          ];
          ask = [
            "Bash(git push:*)"
          ];
          defaultMode = "acceptEdits";
          deny = [
            "WebFetch"
            "Bash(curl:*)"
            "Read(./.env)"
            "Read(./secrets/**)"
          ];
          # disableBypassPermissionsMode = "enable";
        };
        statusLine = {
          command = "npx -y ccusage@latest statusline";
          padding = 0;
          type = "command";
        };
      };
    };

    home.packages = with pkgs; [
      (writeShellScriptBin "claude-litellm" ''
        export ANTHROPIC_BASE_URL=$(cat "${config.sops.secrets.litellm_endpoint.path}")
        exec claude "$@"
      '')
    ];
  };
}
