# Shared home-manager bundle: the feature set every user gets, plus the default
# enable states. Mirrors the old home/shared/default.nix, but composes the
# individually-named feature modules via the dendritic namespace instead of
# importing files by path. Per-user modules flip these defaults as needed.
{ config, lib, ... }:
let
  hm = config.flake.modules.homeManager;
in
{
  flake.modules.homeManager.shared = {
    imports = [
      hm.nixpkgs # allowUnfree + pkgs-stable arg
      hm.zsh
      hm.git
      hm.direnv
      hm.tmux
      hm.stylix
      hm.nvim
      hm.kitty
      hm.zen
      hm.sops
      hm.claude
      hm.codex
      hm.grok
      hm.blender-mcp
    ];

    zsh.enable = lib.mkDefault true;
    direnv.enable = lib.mkDefault true;
    git.enable = lib.mkDefault true;
    tmux.enable = lib.mkDefault true;
    stylix-home.enable = lib.mkDefault true;

    kitty.enable = lib.mkDefault false;

    zen.enable = lib.mkDefault false;

    claude.enable = lib.mkDefault false;
    codex.enable = lib.mkDefault false;
    grok.enable = lib.mkDefault false;
    blender-mcp.enable = lib.mkDefault false;

    sops-home.enable = lib.mkDefault false;
  };
}
