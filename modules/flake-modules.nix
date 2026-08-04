# Declares the dendritic `flake.modules.<class>.<name>` namespace that
# flake-parts core does not provide (it ships only `flake.nixosModules`).
#
# Each entry is a reusable NixOS / nix-darwin / home-manager module, grouped by
# class. Files across the tree contribute to this namespace; configurations then
# select pieces by name via `config.flake.modules.<class>.<name>`. Because the
# leaf type is `deferredModule`, multiple files may define the same name and they
# merge, letting one feature span several files.
{ lib, ... }:
{
  options.flake.modules = lib.mkOption {
    type = lib.types.lazyAttrsOf (lib.types.lazyAttrsOf lib.types.deferredModule);
    default = { };
    description = "Reusable modules grouped by class (nixos, darwin, homeManager).";
  };

  # Export just this option declaration as a flakeModule so a downstream flake
  # (e.g. a private wrapper) can `imports = [ dots.flakeModules.default ]` to get
  # the `flake.modules` namespace, then define its own entries in it — while
  # consuming public's evaluated pieces via `dots.modules.<class>.<name>`.
  config.flake.flakeModules.default = ./flake-modules.nix;
}
