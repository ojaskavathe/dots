{
  config,
  lib,
  pkgs,
  inputs,
  ...
}:

{
  environment = {
    systemPackages = with pkgs; [
    ];
  };

  nixpkgs.config = {
    allowUnfree = true;
  };
  nixpkgs.hostPlatform = "aarch64-darwin";

  nix.settings.experimental-features = [
    "nix-command"
    "flakes"
  ];

  system.stateVersion = 5;
}
