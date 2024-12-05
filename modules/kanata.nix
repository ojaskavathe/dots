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
          config = ''
            (defsrc
              caps   a   s   d   f   j   k   l   ;
            )
            (defvar
              tap-time 200
              hold-time 150
            
              left-hand-keys (
                q w e r t
                a s d f g
                z x c v b
              )
              right-hand-keys (
                y u i o p
                h j k l ;
                n m , . /
              )
            )
            (deflayer base
              @caps  @a  @s  @d  @f  @j  @k  @l  @;
            )
            
            (deflayer nomods
              @caps  a   s   d   f   j   k   l   ;
            )
            (deffakekeys
              to-base (layer-switch base)
            )
            (defalias
              tap (multi
                (layer-switch nomods)
                (on-idle-fakekey to-base tap 20)
              )
            
              caps (tap-hold $tap-time $hold-time esc lctl)
              a (tap-hold-release-keys $tap-time $hold-time (multi a @tap) lmet $left-hand-keys)
              s (tap-hold-release-keys $tap-time $hold-time (multi s @tap) lalt $left-hand-keys)
              d (tap-hold-release-keys $tap-time $hold-time (multi d @tap) lctl $left-hand-keys)
              f (tap-hold-release-keys $tap-time $hold-time (multi f @tap) lsft $left-hand-keys)
              j (tap-hold-release-keys $tap-time $hold-time (multi j @tap) rsft $right-hand-keys)
              k (tap-hold-release-keys $tap-time $hold-time (multi k @tap) rctl $right-hand-keys)
              l (tap-hold-release-keys $tap-time $hold-time (multi l @tap) ralt $right-hand-keys)
              ; (tap-hold-release-keys $tap-time $hold-time (multi ; @tap) rmet $right-hand-keys)
            )
          '';
        };
      };
    };
  };
}
