{
  pkgs,
  lib,
  config,
  ...
}:
{
  options = {
    kitty = {
      enable = lib.mkEnableOption "Enable Kitty";
    };
  };

  config = lib.mkIf config.kitty.enable {
    nixpkgs = {
      overlays = [
        # change icon
        (final: prev: {
          kitty = prev.kitty.overrideAttrs (oldAttrs: {
            postInstall =
              (oldAttrs.postInstall or "")
              + ''
                cp -f ${./kitty.app.png} $out/share/icons/hicolor/256x256/apps/kitty.png
                rm -f $out/share/icons/hicolor/scalable/apps/kitty.svg
              '';
          });
        })

      ];
    };

    programs.kitty = {
      enable = true;
      settings = {
        confirm_os_window_close = 0;
      };
    };
  };
}
