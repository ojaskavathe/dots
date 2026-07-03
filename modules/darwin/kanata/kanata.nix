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
            serviceConfig = {
              ProgramArguments = [
                "/opt/homebrew/bin/kanata"
                "--cfg"
                (toString (pkgs.writeText "hrm.kbd" kanataConfig))
                "--no-wait"
                "-p"
                "5829"
              ];
              KeepAlive = true;
              RunAtLoad = true;
              StandardOutPath = "/tmp/kanata_daemon.out.log";
              StandardErrorPath = "/tmp/kanata_daemon.err.log";
            };
          };

        # kanata's macOS backend does not re-grab a keyboard that (re)connects
        # after it starts — common with Bluetooth keyboards on wake or power-on.
        # This watches the set of connected physical keyboards and kickstarts
        # kanata whenever it changes so newly-connected keyboards get grabbed.
        kanata-watch = {
          serviceConfig = {
            ProgramArguments = [
              (toString (pkgs.writeShellScript "kanata-watch" ''
                # Signature of connected keyboards (UsagePage 1, Usage 6),
                # excluding the Karabiner virtual keyboard (vendor 0x16c0)
                # that kanata recreates on restart — including it would loop.
                sig() {
                  /usr/bin/hidutil list --matching '{"PrimaryUsagePage":1,"PrimaryUsage":6}' 2>/dev/null \
                    | /usr/bin/awk '$1 ~ /^0x/ && $1 != "0x16c0" { print $1":"$2 }' \
                    | /usr/bin/sort -u
                }

                last="$(sig)"
                while true; do
                  /bin/sleep 3
                  cur="$(sig)"
                  if [ "$cur" != "$last" ]; then
                    last="$cur"
                    /bin/launchctl kickstart -k system/org.nixos.kanata
                  fi
                done
              ''))
            ];
            KeepAlive = true;
            RunAtLoad = true;
            StandardOutPath = "/tmp/kanata_watch.out.log";
            StandardErrorPath = "/tmp/kanata_watch.err.log";
          };
        };
      };
    };
  };
}
