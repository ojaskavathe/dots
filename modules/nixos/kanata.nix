{
  lib,
  config,
  ...
}:
{
  options = {
    kanata = {
      enable = lib.mkEnableOption "Remap caps to ctrl and esc";
    };
  };

  config = lib.mkIf config.kanata.enable {
    services.kanata = {
      enable = true;
      keyboards = {
        swapcaps = {
          devices = [
            "/dev/input/by-path/platform-i8042-serio-0-event-kbd" # in-built keyboard
            "/dev/input/by-id/usb-SONiX_AK820-event-kbd" # ak820 (USB cable)
            "/dev/input/by-id/usb-0c45_2.4G_Dongle-event-kbd" # ak820 (USB dongle)
          ];
          extraDefCfg = "process-unmapped-keys yes";
          config = builtins.readFile ../shared/kanata/hrm.kbd;
        };
      };
    };
  };
}
