# Host: galio-wsl (x86_64-linux, WSL, user `dingus`).
# primaryUser via _module.args; allowUnfree from the shared nixpkgs module.
{ ... }:
{
  flake.modules.nixos.galio-wsl =
    {
      config,
      lib,
      pkgs,
      primaryUser,
      ...
    }:
    {
      _module.args.primaryUser = "dingus";

      environment = {
        systemPackages = with pkgs; [
        ];
      };

      nix.settings.experimental-features = [
        "nix-command"
        "flakes"
      ];

      wsl = {
        enable = true;
        defaultUser = primaryUser;
        startMenuLaunchers = true;
      };

      users.mutableUsers = false;
      users.users.dingus = {
        isNormalUser = true;
        description = "dingus";
        hashedPassword = "$6$KjZbzuJytrxrQuCb$UhpJOGUU2GUC4R0hLQig0SkfDTWsVp.dSO/aUYo58r1AYNe34IqUIHIiRitVqkJGKAjSe4NqVywunTjnrarzY/";
        extraGroups = [ "wheel" ];
        shell = pkgs.zsh;
      };
      users.users.root.hashedPassword = "!";
      programs.nix-ld.enable = true;
      programs.zsh.enable = true;

      networking = {
        hostName = "galio-wsl";
      };

      system.stateVersion = "25.05";
    };
}
