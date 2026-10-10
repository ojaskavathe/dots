{ ... }:
{
  flake.modules.homeManager.git =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    {
      options = {
        git = {
          enable = lib.mkEnableOption "Enable Git";
        };
      };

      config = lib.mkIf config.git.enable {
        programs.git = {
          enable = true;
          signing.format = null;
          # identity and aliases live in a plain gitconfig so windows (no nix)
          # can include the same file; see windows/setup.ps1
          includes = [ { path = ./gitconfig; } ];
        };
      };
    };
}
