{
  config,
  lib,
  pkgs,
  inputs,
  ...
}:

{
  imports = [
    # Include the results of the hardware scan.
    (import ./disko-configuration.nix { target = "/dev/nvme0n1"; })
    ./hardware-configuration.nix
  ];

  # Use systemd-boot
  boot.loader.systemd-boot.enable = true;

  # boot.loader.grub = {
  #   enable = true;
  #   useOSProber = true;
  #   efiSupport = true;
  #   device = "nodev";
  # };

  networking.hostName = "tuf"; # Define your hostname.
  networking.networkmanager.enable = true; # Easiest to use and most distros use this by default.

  hardware.bluetooth.enable = true;

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
  };

  security.rtkit.enable = true;
  services.pipewire = {
    enable = true;
    alsa.enable = true;
    alsa.support32Bit = true;
    pulse.enable = true;
    wireplumber.enable = true;
  };

  programs = {
    rog-control-center.enable = true;
  };
  services = {
    asusd = {
      enable = true;
      enableUserService = true;
    };
    supergfxd = {
      enable = true;
      # settings = {
      #   always_reboot = false;
      #   no_logind = true;
      #   mode = "Integrated";
      #   # mode = "Hybrid";
      #   vfio_enable = false;
      #   vfio_save = false;
      #   logout_timeout_s = 180;
      #   hotplug_type = "None";
      # };
    };
  };

  stylix = {
    enable = true;
    image = ../../data/wallpapers/wp.jpg;
    base16Scheme = "${pkgs.base16-schemes}/share/themes/catppuccin-mocha.yaml";
    polarity = "dark";

    cursor = {
      package = pkgs.bibata-cursors;
      name = "Bibata-Modern-Ice";
    };

    fonts = {
      monospace = {
        package = pkgs.nerd-fonts.jetbrains-mono;
        name = "JetBrainsMonoNL Nerd Font Mono";
      };
      sansSerif = {
        package = pkgs.rubik;
        name = "Rubik";
      };
    };
  };

  hardware.i2c.enable = true; # for ddcutil
  services.udev.extraRules = ''
    KERNEL=="i2c-[0-9]*", GROUP="i2c", MODE="0660"
  '';

  programs.zsh.enable = true;
  users.mutableUsers = false;
  users.users.dingus = {
    isNormalUser = true;
    description = "dingus";
    hashedPassword = "$6$KjZbzuJytrxrQuCb$UhpJOGUU2GUC4R0hLQig0SkfDTWsVp.dSO/aUYo58r1AYNe34IqUIHIiRitVqkJGKAjSe4NqVywunTjnrarzY/";
    extraGroups = [
      "network-manager"
      "wheel"
    ];
    shell = pkgs.zsh;
  };
  # disabling the root user
  users.users.root.hashedPassword = "!";

  programs.steam = {
    enable = true;
    remotePlay.openFirewall = true;
    dedicatedServer.openFirewall = true;
    localNetworkGameTransfers.openFirewall = true;
  };

  programs.firefox.enable = true;
  programs.ssh.startAgent = true;

  environment = {
    systemPackages = with pkgs; [
      vim
      (catppuccin-sddm.override {
        flavor = "mocha";
        # font  = "Rubik";
        # fontSize = "9";
        # background = "${./wallpaper.png}";
        # loginBackground = true;
      })
      lxqt.lxqt-policykit
    ];
  };
  environment.sessionVariables = {
    XDG_CACHE_HOME = "$HOME/.cache";
    XDG_CONFIG_HOME = "$HOME/.config";
    XDG_DATA_HOME = "$HOME/.local/share";
    XDG_STATE_HOME = "$HOME/.local/state";
  };

  security.polkit.enable = true;

  nixpkgs.config = {
    allowUnfree = true;
  };

  nvidia = {
    enable = true;
    optimus = true;
  };

  kanata.enable = true;

  hyprland.enable = true;

  services.cloudflare-warp.enable = true;

  nix.settings.experimental-features = [
    "nix-command"
    "flakes"
  ];

  system.stateVersion = "24.05";

  #virtualisation.vmware.guest.enable = true;
  virtualisation.docker = {
    enable = true;
    storageDriver = "btrfs";
  };
}
