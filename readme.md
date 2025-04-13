# NixOS and Darwin Configuration

This repository contains my Nix configurations for both NixOS and Darwin (macOS) systems, managed through Nix flakes. It includes home-manager configurations and various system modules.

## System Configurations

### NixOS (TUF)
The NixOS configuration was designed for my Asus TUF A15 (tuf) and includes:
- Disko-based disk partitioning and formatting
- System-wide configurations in `hosts/tuf/`
- Shared and NixOS-specific modules

### Darwin (Camille)
The Darwin configuration is designed for an M4 Max Macbook Pro (camille) and includes:
- System-wide configurations in `hosts/camille/`
- Shared and Darwin-specific modules
- Homebrew integration through nix-homebrew

## Nixos/Darwin Modules

```
modules/
├── shared/
│   └── default.nix
├── nixos/
│   ├── default.nix
│   ├── hyprland.nix
│   ├── kanata.nix
│   └── nvidia.nix
└── darwin/
    ├── default.nix
    ├── aerospace.nix
    ├── homebrew.nix
    └── kanata/
```

## Home Manager Modules
```
home/
├── default.nix
├── git.nix
├── stylix-home.nix
├── tmux.nix
├── desktop/
│   ├── hyprland/
│   └── kde.nix
├── kitty/
│   ├── kitty.nix
│   └── kitty.app.png
├── nvim/
│   ├── nvim.nix
│   ├── diagnostics.lua
│   ├── keymap.lua
│   ├── options.lua
│   ├── plugins/
│   └── statuscol.lua
└── shell/
    ├── zsh.nix
    ├── aliases.zsh
    ├── completions.zsh
    └── direnv.nix
```

## User Configurations

User-specific configurations are stored in the `users/` directory:
- `dingus.nix`: Configuration for the dingus user on NixOS
- `ojas.nix`: Configuration for the ojas user on Darwin

## Installation

### NixOS Installation

1. Clone the repository in `/tmp`:
   ```bash
   git clone <repository-url> /tmp/nixos-config
   ```

2. Partition and format using disko:
   ```bash
   sudo nix --experimental-features "nix-command flakes" run github:nix-community/disko -- --mode disko --argstr target <device-name> /tmp/nixos-config/nixos/disko-configuration.nix
   ```

3. Generate and set up configuration:
   ```bash
   sudo nixos-generate-config --no-filesystems --root /mnt
   sudo rm /mnt/etc/nixos/configuration.nix
   sudo mv /tmp/nixos-config/* /mnt/etc/nixos
   sudo mv /mnt/etc/nixos/hardware-configuration /mnt/etc/nixos/nixos
   ```

4. Install NixOS:
   ```bash
   sudo nixos-install -v --show-trace --no-root-passwd --root /mnt --flake /mnt/etc/nixos#nixos
   ```

### Darwin Installation

1. Install nix-darwin:
   ```bash
   nix-build https://github.com/LnL7/nix-darwin/archive/master.tar.gz -A installer
   ./result/bin/darwin-installer
   ```

2. Clone the repository and switch to the configuration:
   ```bash
   git clone <repository-url> ~/.config/nixpkgs
   darwin-rebuild switch --flake ~/.config/nixpkgs#camille
   ```

## Home Manager

To apply home-manager configurations:

### NixOS
```bash
home-manager switch --flake .#dingus@nixos
```

### Darwin
```bash
home-manager switch --flake .#ojas@camille
```

- Install home-manager using `nix shell nixpkgs#home-manager`.

## Misc

### Kanata on MacOS

Install Karabiner-DriverKit-VirtualHIDDevice from [here](https://github.com/pqrs-org/Karabiner-DriverKit-VirtualHIDDevice/tree/main/dist).

Then run:
```
/Applications/.Karabiner-VirtualHIDDevice-Manager.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Manager activate
```

Then go to Settings > General > Driver Extensions and enable the VirtualHIDDevice.

On restart, the launchctl daemon should run an instance of kanata. For errors, check in `/tmp`.
