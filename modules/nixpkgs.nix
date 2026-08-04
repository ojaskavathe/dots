# Shared nixpkgs configuration, exposed as a named module for every class.
#
# Replaces two bits of per-config boilerplate that used to be copy-pasted into
# each host/user file and threaded through specialArgs:
#   - `nixpkgs.config.allowUnfree` (+ predicate)
#   - `pkgs-stable`: a nixpkgs-26.05 instance, injected as a module arg so any
#     module can take `pkgs-stable` without it riding through specialArgs.
#
# allowUnfree only gates access, so consolidating it here changes no derivation.
# pkgs-stable is built from the same input + config as before, so packages drawn
# from it (harlequin, awscli2, tmux resurrect) keep identical store paths.
{ inputs, ... }:
let
  common =
    { pkgs, ... }:
    {
      nixpkgs.config = {
        allowUnfree = true;
        allowUnfreePredicate = _: true;
      };

      _module.args.pkgs-stable = import inputs.nixpkgs-stable {
        inherit (pkgs.stdenv.hostPlatform) system;
        config = {
          allowUnfree = true;
          allowUnfreePredicate = _: true;
        };
      };
    };
in
{
  flake.modules.nixos.nixpkgs = common;
  flake.modules.darwin.nixpkgs = common;
  flake.modules.homeManager.nixpkgs = common;
}
