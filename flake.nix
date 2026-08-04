{

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-26.05";

    flake-utils.url = "github:numtide/flake-utils";

    flake-parts.url = "github:hercules-ci/flake-parts";
    import-tree.url = "github:vic/import-tree";

    nix-darwin = {
      url = "github:LnL7/nix-darwin/master";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nix-homebrew = {
      url = "github:zhaofengli-wip/nix-homebrew";
      # nix-homebrew's own brew pin drifts out of sync with the homebrew-core
      # /cask pins below and breaks `brew bundle` on formulae using newer DSL.
      # track brew's default branch (what a normal `brew update` follows) so
      # all three move together.
      inputs.brew-src.url = "github:Homebrew/brew";
    };

    homebrew-core = {
      url = "github:homebrew/homebrew-core";
      flake = false;
    };

    homebrew-cask = {
      url = "github:homebrew/homebrew-cask";
      flake = false;
    };

    homebrew-bundle = {
      url = "github:homebrew/homebrew-bundle";
      flake = false;
    };

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    plasma-manager = {
      url = "github:nix-community/plasma-manager";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.home-manager.follows = "home-manager";
    };

    disko = {
      url = "github:nix-community/disko";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nixos-wsl = {
      url = "github:nix-community/NixOS-WSL";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # hyprland.url = "github:hyprwm/Hyprland?submodules=1";

    stylix = {
      url = "github:nix-community/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    sops-nix = {
      url = "github:mic92/sops-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    secrets = {
      url = "git+ssh://git@github.com/ojaskavathe/secrets.git";
      flake = false;
    };

    nvim = {
      url = "path:./home/shared/nvim";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    zen-browser = {
      # Using fork with macOS profile path fix
      # See: https://github.com/ReeSilva/zen-browser-flake/pull/1
      url = "github:0xc000022070/zen-browser-flake";
      # url = "github:ReeSilva/zen-browser-flake";
      # IMPORTANT: we're using "libgbm" and is only available in unstable so ensure
      # to have it up-to-date or simply don't specify the nixpkgs input
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  # Migration to the dendritic pattern (flake-parts + import-tree) is in
  # progress. Phase 0: everything still lives verbatim in the `flake` block
  # below; only the outer scaffolding changed. Later phases move each config
  # into per-feature flake-parts modules under ./modules.
  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } (
      { config, ... }:
      let
        inherit (inputs)
          disko
          nixpkgs
          nixpkgs-stable
          nix-darwin
          home-manager
          stylix
          ;
        hm = config.flake.modules.homeManager;
        dm = config.flake.modules.darwin;
      in
      {
        # Converted flake-parts modules are listed explicitly during the
        # migration so unconverted files under ./modules stay untouched. Phase 5
        # swaps this whole list for `inputs.import-tree ./modules`.
        imports = [
          ./modules/flake/modules-option.nix
          ./modules/lib/nixpkgs.nix
          ./modules/home/git.nix
          ./modules/home/zsh.nix
          ./modules/home/direnv.nix
          ./modules/home/tmux.nix
          ./modules/home/stylix.nix
          ./modules/home/kitty.nix
          ./modules/home/zen.nix
          ./modules/home/sops.nix
          ./modules/home/claude.nix
          ./modules/home/codex.nix
          ./modules/home/blender-mcp.nix
          ./modules/home/kde.nix
          ./modules/home/hyprland.nix
          ./modules/home/shared.nix
          ./modules/home/linux.nix
          ./modules/home/ojas.nix
          ./modules/home/dingus-wsl.nix
          ./modules/home/dingus.nix
          ./modules/darwin/aerospace.nix
          ./modules/darwin/homebrew.nix
          ./modules/darwin/kanata/kanata.nix
          ./modules/darwin/base.nix
          ./modules/darwin/camille.nix
        ];

        systems = [
          "aarch64-darwin"
          "x86_64-linux"
        ];

        # for ags / hyprland stuff
        perSystem =
          { pkgs, ... }:
          {
            devShells.default = pkgs.mkShell {
              packages = with pkgs; [
                just
                nixfmt
                stylua
                shfmt
                nodejs
              ];
            };
          };

        flake = {

          diskoConfigurations.nixos = import ./hosts/tuf/disko-configuration.nix;

          nixosConfigurations =
            let
              system = "x86_64-linux";
            in
            {

              tuf = nixpkgs.lib.nixosSystem {
                inherit system;
                specialArgs = {
                  inherit inputs system;
                };
                modules = [
                  ./hosts/tuf/configuration.nix
                  ./modules/nixos
                  disko.nixosModules.disko
                  stylix.nixosModules.stylix
                ];
              };

              galio-wsl = nixpkgs.lib.nixosSystem {
                inherit system;
                specialArgs = {
                  inherit inputs system;
                  primaryUser = "dingus";
                };
                modules = [
                  inputs.nixos-wsl.nixosModules.default
                  ./hosts/galio-wsl/configuration.nix
                  stylix.nixosModules.stylix
                ];
              };

            };

          darwinConfigurations = {

            camille = nix-darwin.lib.darwinSystem {
              modules = [
                inputs.nix-homebrew.darwinModules.nix-homebrew
                dm.nixpkgs
                dm.base
                dm.camille
              ];
            };

          };

          homeConfigurations = {

            "dingus@tuf" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages."x86_64-linux";
              modules = [
                inputs.plasma-manager.homeManagerModules.plasma-manager
                stylix.homeModules.stylix
                hm.shared
                hm.linux
                hm.dingus
              ];
            };

            "dingus@galio-wsl" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages."x86_64-linux";
              modules = [
                stylix.homeModules.stylix
                inputs.zen-browser.homeModules.beta
                inputs.nvim.homeModule
                hm.shared
                hm.dingus-wsl
              ];
            };

            "ojas@camille" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages."aarch64-darwin";
              modules = [
                stylix.homeModules.stylix
                inputs.zen-browser.homeModules.beta
                inputs.nvim.homeModule
                hm.shared
                hm.ojas
              ];
            };

          };

        };
      }
    );
}
