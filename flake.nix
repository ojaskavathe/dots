{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-24.11";

    flake-utils.url = "github:numtide/flake-utils";

    nix-darwin = {
      url = "github:LnL7/nix-darwin/master";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nix-homebrew.url = "github:zhaofengli-wip/nix-homebrew";
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

    # hyprland.url = "github:hyprwm/Hyprland?submodules=1";

    stylix = {
      url = "github:danth/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      disko,
      nixpkgs,
      nixpkgs-stable,
      nix-darwin,
      home-manager,
      stylix,
      ...
    }@inputs:
    let
      inherit (self) outputs;
    in
    {
      diskoConfigurations.nixos = import ./hosts/tuf/disko-configuration.nix;

      nixosConfigurations =
        let
          system = "x86_64-linux";
          pkgs = nixpkgs.legacyPackages.${system};
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
        };

      darwinConfigurations =
        let
          system = "aarch64-darwin";
          pkgs = nixpkgs.legacyPackages.${system};
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
              ./home
              ./users/dingus.nix
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
              inputs.plasma-manager.homeManagerModules.plasma-manager
              ./home
              ./users/ojas.nix
            ];
          };
      };
    }
    # for ags / hyprland stuff
    // inputs.flake-utils.lib.eachDefaultSystem (system: {
      devShells.default =
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        pkgs.mkShell {
          packages = with pkgs; [
            nodejs
          ];
        };
    });
}
