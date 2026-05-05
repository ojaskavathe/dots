{
  lib,
  config,
  pkgs,
  ...
}:
{
  options = {
    direnv = {
      enable = lib.mkEnableOption "Enable Direnv";
    };
  };

  config = lib.mkIf config.direnv.enable {
    programs.direnv = {
      enable = true;
      package = pkgs.direnv.overrideAttrs { doCheck = false; };
      enableZshIntegration = true;
      nix-direnv.enable = true;
    };
  };
}
