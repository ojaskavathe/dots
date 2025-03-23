{
  pkgs,
  pkgs-stable,
  lib,
  config,
  ...
}:
{

  options = {
    kanata-macos = {
      enable = lib.mkEnableOption "Enable Kanata on MacOS";
    };
  };

  config = lib.mkIf config.kanata-macos.enable {
    xdg.configFile = {
      "kanata/hrm.kbd".source = ./hrm.kbd;
    };
  };
}
