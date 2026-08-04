# Neovim configuration, wrapped with nix-wrapper-modules.
#
# A standalone flake. It can be:
#   1. Run directly:              nix run .
#   2. Run from the dots repo:    nix run ~/dots/modules/_nvim
#   3. Integrated with home-mgr:  inputs.nvim.homeModule (see nix/home-module.nix)
#
# How it works:
#   - nix/module.nix defines the wrapper: which plugins load at startup vs lazily
#     (config.specs.*) and which tools go on nvim's PATH (runtimePkgs).
#   - lua/ + init.lua are the actual neovim config, baked into the derivation
#     (config.settings.config_directory), so `nix run` works anywhere.
#   - the lua talks to nix through the info plugin; a small shim in
#     lua/config/init.lua keeps the old nixCats("cat") / for_cat calls working.
#
# See: https://birdeehub.github.io/nix-wrapper-modules/wrapperModules/neovim.html
{
  description = "Neovim configuration wrapped with nix-wrapper-modules and lze lazy-loading";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    wrappers.url = "github:BirdeeHub/nix-wrapper-modules";
    wrappers.inputs.nixpkgs.follows = "nixpkgs";

    # Fetched fresh (not via nixpkgs) so they track upstream. Auto-detected by
    # the "plugins-" prefix and exposed as config.nvim-lib.neovimPlugins.<name>.
    "plugins-lze" = {
      url = "github:BirdeeHub/lze"; # lazy-loading engine
      flake = false;
    };
    "plugins-lzextras" = {
      url = "github:BirdeeHub/lzextras"; # extra lze handlers (lsp, etc.)
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      wrappers,
      ...
    }@inputs:
    let
      forAllSystems = nixpkgs.lib.genAttrs nixpkgs.lib.platforms.all;
      # Pre-apply the flake inputs to the wrapper module, then evaluate it.
      module = nixpkgs.lib.modules.importApply ./nix/module.nix inputs;
      wrapper = wrappers.lib.evalModule module;
    in
    {
      # The evaluated wrapper — downstream can `.wrap { pkgs }` it.
      wrapperModules.default = module;
      wrappers.default = wrapper.config;

      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            config.allowUnfree = true;
          };
        in
        rec {
          nvim = wrapper.config.wrap { inherit pkgs; };
          default = nvim;
        }
      );

      # Home-manager module: programs.nvim (enable/package/aliases/defaultEditor).
      homeModule = import ./nix/home-module.nix { inherit self; };

      # NixOS / nix-darwin install module (alternative to home-manager).
      nixosModules.default = wrappers.lib.getInstallModule {
        name = "nvim";
        value = module;
      };
    };
}
