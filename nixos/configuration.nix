{ config, lib, pkgs, ... }:
 
{
  imports =
    [ # Include the results of the hardware scan.
      ./disko-configuration.nix
      ./hardware-configuration.nix
    ];

  # Use systemd-boot
  boot.loader.systemd-boot.enable = true;
  
  # Use the GRUB 2 boot loader.
  # boot.loader.grub.enable = true;
  # boot.loader.grub.efiSupport = true;
  # boot.loader.grub.device = "nodev" # for efi only
  # boot.loader.grub.useOSProber = true;
 
  networking.hostName = "nixos"; # Define your hostname.
  networking.networkmanager.enable = true;  # Easiest to use and most distros use this by default.
 
  # Set your time zone.
  time.timeZone = "Asia/Kolkata";
 
  # Select internationalisation properties.
  i18n.defaultLocale = "en_US.UTF-8";
 
  # Enable the X11 windowing system.
  services = {
    xserver.enable = true;

    displayManager = {
      sddm = {
        enable = true;
        theme = "catppuccin-mocha";
        # package = pkgs.kdePackages.sddm;
        settings = {
          General = {
            InputMethod = "";
          };
        };
      };
    };
    desktopManager.plasma6.enable = true;

    # Configure keymap in X11
    xserver.xkb.layout = "us";
    xserver.xkb.options = "";

    # Enable CUPS to print documents.
    printing.enable = true;

    # Enable sound.
    pipewire = {
      enable = true;
      alsa.enable = true;
      alsa.support32Bit = true;
      pulse.enable = true;
    };
  };

  programs.zsh.enable = true;
  users.mutableUsers = false;
  users.users.dingus = {
    isNormalUser = true;
    description = "dingus";
    hashedPassword = "$6$KjZbzuJytrxrQuCb$UhpJOGUU2GUC4R0hLQig0SkfDTWsVp.dSO/aUYo58r1AYNe34IqUIHIiRitVqkJGKAjSe4NqVywunTjnrarzY/";
    extraGroups = [ "network-manager" "wheel" ];
    shell = pkgs.zsh;
  };
  # disabling the root user
  users.users.root.hashedPassword = "!";

  programs.firefox.enable = true;
  programs.ssh.startAgent = true;

  environment = {
    systemPackages = with pkgs; [
      vim
      (catppuccin-sddm.override {
        flavor = "mocha";
        # font  = "Noto Sans";
        # fontSize = "9";
        # background = "${./wallpaper.png}";
        # loginBackground = true;
      })
    ];
    plasma6.excludePackages = with pkgs.kdePackages; [
      kwallet
    ];
  };

  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  system.stateVersion = "24.05";

  #virtualisation.vmware.guest.enable = true;
}
