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
      { ... }:
      let
        inherit (inputs)
          disko
          nixpkgs
          nixpkgs-stable
          nix-darwin
          home-manager
          stylix
          ;
      in
      {
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
                  ./modules/shared
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
                  ./modules/shared
                  stylix.nixosModules.stylix
                ];
              };

            };

          darwinConfigurations =
            let
              system = "aarch64-darwin";
              primaryUser = "ojas"; # single-user
            in
            {

              camille = nix-darwin.lib.darwinSystem {
                specialArgs =
                  let
                    pkgs-stable = import nixpkgs-stable {
                      inherit system;
                      config = {
                        allowUnfree = true;
                        allowUnfreePredicate = (_: true);
                      };
                    };
                  in
                  {
                    inherit
                      inputs
                      system
                      pkgs-stable
                      primaryUser
                      ;
                  };
                modules = [
                  inputs.nix-homebrew.darwinModules.nix-homebrew
                  ./hosts/camille/configuration.nix
                  ./modules/shared
                  ./modules/darwin
                ];
              };

            };

          homeConfigurations = {

            "dingus@tuf" =
              let
                system = "x86_64-linux";
              in
              home-manager.lib.homeManagerConfiguration {
                pkgs = nixpkgs.legacyPackages.${system}; # Home-manager requires 'pkgs' instance
                extraSpecialArgs =
                  let
                    pkgs-stable = import nixpkgs-stable {
                      inherit system;
                      config = {
                        allowUnfree = true;
                        allowUnfreePredicate = (_: true);
                      };
                    };
                  in
                  {
                    inherit pkgs-stable inputs system;
                  };
                modules = [
                  inputs.plasma-manager.homeManagerModules.plasma-manager
                  stylix.homeModules.stylix
                  ./home/shared
                  ./home/nixos
                  ./users/dingus.nix
                ];
              };

            "dingus@galio-wsl" =
              let
                system = "x86_64-linux";
                username = "dingus";
              in
              home-manager.lib.homeManagerConfiguration {
                pkgs = nixpkgs.legacyPackages.${system};
                extraSpecialArgs =
                  let
                    pkgs-stable = import nixpkgs-stable {
                      inherit system;
                      config = {
                        allowUnfree = true;
                        allowUnfreePredicate = (_: true);
                      };
                    };
                  in
                  {
                    inherit
                      pkgs-stable
                      inputs
                      system
                      username
                      ;
                  };
                modules = [
                  stylix.homeModules.stylix
                  inputs.zen-browser.homeModules.beta
                  inputs.nvim.homeModule
                  ./home/shared
                  ./users/dingus-wsl.nix
                ];
              };

            "ojas@camille" =
              let
                system = "aarch64-darwin";
                username = "ojas";
              in
              home-manager.lib.homeManagerConfiguration {
                pkgs = nixpkgs.legacyPackages."aarch64-darwin"; # Home-manager requires 'pkgs' instance
                extraSpecialArgs =
                  let
                    pkgs-stable = import nixpkgs-stable {
                      inherit system;
                      config = {
                        allowUnfree = true;
                        allowUnfreePredicate = (_: true);
                      };
                    };
                  in
                  {
                    inherit
                      pkgs-stable
                      inputs
                      system
                      username
                      ;
                  };
                modules = [
                  stylix.homeModules.stylix
                  inputs.zen-browser.homeModules.beta
                  inputs.nvim.homeModule
                  ./home/shared
                  ./home/darwin
                  ./users/ojas.nix
                ];
              };

          };

        };
      }
    );
}
