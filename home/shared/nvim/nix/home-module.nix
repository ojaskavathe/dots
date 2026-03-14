# Home-manager module for this neovim config.
#
# Usage in your main flake:
#   1. Add as input:   nvim.url = "path:./home/shared/nvim";
#   2. Import module:  inputs.nvim.homeModule
#   3. Enable in user config:
#        programs.nvim = {
#          enable = true;
#          aliases = [ "vim" "vi" ];
#        };
#
# This puts the nvim binary on PATH and optionally sets EDITOR/VISUAL
# and shell aliases.

{ self }:
{ config, lib, pkgs, ... }:
let
  cfg = config.programs.nvim;
  inherit (lib) mkEnableOption mkOption mkIf types;
in
{
  options.programs.nvim = {
    enable = mkEnableOption "Neovim";

    package = mkOption {
      type = types.package;
      default = self.packages.${pkgs.system}.default;
      description = "The neovim package to use";
    };

    defaultEditor = mkOption {
      type = types.bool;
      default = true;
      description = "Set EDITOR and VISUAL to nvim";
    };

    aliases = mkOption {
      type = types.listOf types.str;
      default = [ "vi" "vim" ];
      description = "Shell aliases that point to nvim";
    };
  };

  config = mkIf cfg.enable {
    home.packages = [ cfg.package ];

    home.sessionVariables = mkIf cfg.defaultEditor {
      EDITOR = "nvim";
      VISUAL = "nvim";
    };

    # Create aliases in all common shells
    programs.bash.shellAliases = mkIf (cfg.aliases != [ ]) (
      lib.genAttrs cfg.aliases (_: "nvim")
    );
    programs.zsh.shellAliases = mkIf (cfg.aliases != [ ]) (
      lib.genAttrs cfg.aliases (_: "nvim")
    );
    programs.fish.shellAliases = mkIf (cfg.aliases != [ ]) (
      lib.genAttrs cfg.aliases (_: "nvim")
    );
  };
}
