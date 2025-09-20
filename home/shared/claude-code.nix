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
        apiKeyHelper = "${pkgs.writeShellScript "generate_temp_api_key.sh" ''
          cat "${config.sops.secrets.litellm_api_key.path}"
        ''}";
      };
    };

    programs.zsh = {
      initContent = lib.mkOrder 2000 ''
        export ANTHROPIC_BASE_URL=$(cat ${config.sops.secrets.litellm_endpoint.path})
      '';
    };
  };
}
