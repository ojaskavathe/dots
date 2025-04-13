{
  lib,
  config,
  pkgs,
  ...
}:
{
  options = {
    kanata = {
      enable = lib.mkEnableOption "Enable kanata";
    };
  };

  config = lib.mkIf config.kanata.enable {
    launchd = {
      daemons = {
        karabiner-driver = {
          command = ''
            /Library/Application\ Support/org.pqrs/Karabiner-DriverKit-VirtualHIDDevice/Applications/Karabiner-VirtualHIDDevice-Daemon.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Daemon
          '';
          serviceConfig = {
            KeepAlive = true;
            RunAtLoad = true;
            StandardOutPath = "/tmp/Karabiner-VirtualHIDDevice-Daemon.out.log";
            StandardErrorPath = "/tmp/Karabiner-VirtualHIDDevice-Daemon.err.log";
          };
        };

        kanata =
          let
            kanataConfig = builtins.readFile ./hrm.kbd;
          in
          {
            command = ''
              /opt/homebrew/bin/kanata --cfg ${pkgs.writeText "hrm.kbd" kanataConfig} -p 5829
            '';
            serviceConfig = {
              KeepAlive = true;
              RunAtLoad = true;
              StandardOutPath = "/tmp/kanata_daemon.out.log";
              StandardErrorPath = "/tmp/kanata_daemon.err.log";
            };
          };
      };
    };
  };
}
