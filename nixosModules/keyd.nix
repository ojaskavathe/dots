{
  lib,
  config,
  ...
}:
{
  options = {
    keyd = {
      enable = lib.mkEnableOption "Remap caps to ctrl and esc";
    };
  };

  config = lib.mkIf config.keyd.enable {
    services.keyd = {
      enable = true;
      keyboards = {
        default = {
          ids = [
            "0001:0001" # laptop keyboard
            "0c45:8009" # AK820
          ];
          settings = {
            main = {
              capslock = "overload(control, esc)";
              esc = "capslock";
            };
          };
        };
      };
    };
  };
}
