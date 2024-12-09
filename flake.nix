{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-24.11";

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
    ags = {
      url = "github:Aylur/ags/v1";
      # url = "github:Aylur/ags";
      # inputs.nixpkgs.follows = "nixpkgs";
    };

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
      home-manager,
      ags,
      stylix,
      ...
    }@inputs:
    let

      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      inherit (self) outputs;

    in
    {
      diskoConfigurations.nixos = import ./hosts/tuf/disko-configuration.nix;

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
            stylix.nixosModules.stylix ./hosts/tuf/configuration.nix
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
              inherit pkgs-stable inputs system;
            };
          modules = [
            inputs.plasma-manager.homeManagerModules.plasma-manager
            inputs.ags.homeManagerModules.default
            stylix.homeManagerModules.stylix ./users/dingus.nix
            ./home
            ./users/dingus.nix
          ];
        };
      };

      # for work on the flake
      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          nodejs

          # lsp etc.
          nodePackages.typescript-language-server
          nodePackages.prettier
          tailwindcss-language-server
          vscode-langservers-extracted
        ];
      };
    };
}
