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
          disableBypassPermissionsMode = "disable";
        };
        statusLine = {
          command = "input=$(cat); echo \"[$(echo \"$input\" | jq -r '.model.display_name')] 📁 $(basename \"$(echo \"$input\" | jq -r '.workspace.current_dir')\")\"";
          padding = 0;
          type = "command";
        };
      };
    };

    home.packages = with pkgs; [
      ccusage
    ];

    # programs.zsh = {
    #   initContent = lib.mkOrder 2000 ''
    #     export ANTHROPIC_BASE_URL=$(cat ${config.sops.secrets.litellm_endpoint.path})
    #   '';
    # };
  };
}
