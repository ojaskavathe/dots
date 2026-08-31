{

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-26.05";

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

    # tmux-native agent dashboard. follows matters: winch bakes in
    # ${pkgs.tmux}/bin/tmux, which must be the same tmux this config runs.
    winch = {
      url = "github:ojaskavathe/winch";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nvim = {
      url = "path:./modules/_nvim";
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

  # Dendritic pattern: every .nix under ./modules (except paths containing /_)
  # is a flake-parts module, auto-imported by import-tree. Each contributes to
  # the flake.modules.<class>.<name> namespace; the assemblies in the `flake`
  # block below select those pieces by name.
  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } (
      { config, ... }:
      let
        inherit (inputs)
          disko
          nixpkgs
          nix-darwin
          home-manager
          stylix
          ;
        hm = config.flake.modules.homeManager;
        dm = config.flake.modules.darwin;
        nm = config.flake.modules.nixos;
      in
      {
        imports = [
          (inputs.import-tree ./modules)
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

          diskoConfigurations.nixos = import ./modules/hosts/_tuf/disko-configuration.nix;

          nixosConfigurations = {

            tuf = nixpkgs.lib.nixosSystem {
              system = "x86_64-linux";
              modules = [
                nm.tuf
                nm.base
                disko.nixosModules.disko
                stylix.nixosModules.stylix
              ];
            };

            galio-wsl = nixpkgs.lib.nixosSystem {
              system = "x86_64-linux";
              modules = [
                inputs.nixos-wsl.nixosModules.default
                nm.nixpkgs
                nm.galio-wsl
                stylix.nixosModules.stylix
              ];
            };

          };

          darwinConfigurations = {

            camille = nix-darwin.lib.darwinSystem {
              modules = [
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
                hm.shared
                hm.linux
                hm.dingus
              ];
            };

            "dingus@galio-wsl" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages."x86_64-linux";
              modules = [
                hm.shared
                hm.dingus-wsl
              ];
            };

            "ojas@camille" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages."aarch64-darwin";
              modules = [
                hm.shared
                hm.ojas
              ];
            };

          };

        };
      }
    );
}
