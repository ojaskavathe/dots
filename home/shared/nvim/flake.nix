# Neovim configuration using nixCats
#
# This is a standalone flake. It can be:
#   1. Run directly:              nix run .
#   2. Run from the dots repo:    nix run ~/dots/home/shared/nvim
#   3. Run from GitHub:           nix run github:ojaskavathe/dots?dir=home/shared/nvim
#   4. Integrated with home-mgr:  inputs.nvim.homeModule (see home-module.nix)
#
# How nixCats works:
#   - categories.nix defines WHAT plugins/tools are available, grouped by category
#   - packages.nix defines WHICH categories are enabled for a given build
#   - lua/ contains the actual neovim config (init.lua, keymaps, plugin specs)
#   - nixCats bakes it all into a single nix derivation (binary + config + plugins + LSPs)
#
# The lua config can check nixCats("category.name") at runtime to know
# which categories are enabled, so the same lua works for different builds.
#
# See: https://nixcats.org/

{
  description = "Neovim configuration with nixCats and lze lazy-loading";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixCats.url = "github:BirdeeHub/nixCats-nvim";

    # Inputs prefixed with "plugins-" are auto-detected by nixCats'
    # standardPluginOverlay and made available as pkgs.neovimPlugins.*
    "plugins-lze" = {
      url = "github:BirdeeHub/lze"; # Lazy-loading engine
      flake = false;
    };
    "plugins-lzextras" = {
      url = "github:BirdeeHub/lzextras"; # Extra lze handlers (lsp, etc.)
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixCats,
      ...
    }@inputs:
    let
      inherit (nixCats) utils;

      # The root of the lua config. nixCats bundles this into the derivation
      # when wrapRc = true (set in packages.nix)
      luaPath = "${./.}";

      forEachSystem = utils.eachSystem nixpkgs.lib.platforms.all;
      extra_pkg_config = {
        allowUnfree = true;
      };

      # standardPluginOverlay turns "plugins-*" inputs into neovim plugins
      # accessible as pkgs.neovimPlugins.lze, pkgs.neovimPlugins.lzextras, etc.
      dependencyOverlays = [ (utils.standardPluginOverlay inputs) ];

      categoryDefinitions = import ./nix/categories.nix; # what's available
      packageDefinitions = import ./nix/packages.nix; # what's enabled
      defaultPackageName = "nvim"; # also the binary name
    in

    # Per-system outputs (packages, devShells)
    forEachSystem (
      system:
      let
        nixCatsBuilder = utils.baseBuilder luaPath {
          inherit
            nixpkgs
            system
            dependencyOverlays
            extra_pkg_config
            ;
        } categoryDefinitions packageDefinitions;

        defaultPackage = nixCatsBuilder defaultPackageName;
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages = utils.mkAllWithDefault defaultPackage;

        devShells.default = pkgs.mkShell {
          name = defaultPackageName;
          packages = [
            defaultPackage
            pkgs.nil
            pkgs.nixfmt-rfc-style
            pkgs.lua-language-server
          ];
          shellHook = ''echo "Run 'nvim' to start neovim"'';
        };
      }
    )
    // {
      # System-independent outputs

      # Home-manager module: import in your home config to get programs.nvim options
      homeModule = import ./nix/home-module.nix { inherit self; };

      # NixOS/nix-darwin module (alternative to home-manager)
      nixosModules.default = utils.mkNixosModules {
        inherit
          defaultPackageName
          dependencyOverlays
          luaPath
          categoryDefinitions
          packageDefinitions
          extra_pkg_config
          nixpkgs
          ;
      };

      overlays = utils.makeOverlays luaPath {
        inherit nixpkgs dependencyOverlays extra_pkg_config;
      } categoryDefinitions packageDefinitions defaultPackageName;

      inherit utils;
      inherit (utils) templates;
    };
}
