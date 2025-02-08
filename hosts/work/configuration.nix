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

  security.pam.enableSudoTouchIdAuth = true;

  system.defaults = {
    NSGlobalDomain = {
      # ctrl + cmd + mouse to drag windows from any part of the window (not just titlebar)
      NSWindowShouldDragOnGesture = true;
    };
  };

  aerospace = {
    enable = true;
  };
}
