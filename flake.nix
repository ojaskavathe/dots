{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-24.05";

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

    hyprland = {
      url = "github:hyprwm/Hyprland";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    ags = {
      url = "github:Aylur/ags";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      disko,
      nixpkgs,
      nixpkgs-stable,
      home-manager,
      ...
    }@inputs:
    let

      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      inherit (self) outputs;

    in
    {
      diskoConfigurations.nixos = import ./disk-config.nix;

      nixosConfigurations = {
        tuf = nixpkgs.lib.nixosSystem {
          inherit system;
          specialArgs = {
            inherit inputs;
          };
          modules = [
            ./hosts/tuf/configuration.nix
            ./modules
            disko.nixosModules.disko
          ];
        };
      };

      homeConfigurations = {
        "dingus@tuf" = home-manager.lib.homeManagerConfiguration {
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
              inherit pkgs-stable;
            };
          modules = [
            inputs.plasma-manager.homeManagerModules.plasma-manager
            inputs.ags.homeManagerModules.default
            ./home
            ./users/dingus.nix
          ];
        };
      };
    };
}
