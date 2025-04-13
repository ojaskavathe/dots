Clone the repository in /tmp, then partition and format using disko:

```
sudo nix --experimental-features "nix-command flakes" run github:nix-community/disko -- --mode disko --argstr target <device-name> /tmp/nixos-config/nixos/disko-configuration.nix
```

Setup config:

```
sudo nixos-generate-config --no-filesystems --root /mnt
```
```
sudo rm /mnt/etc/nixos/configuration.nix
```
```
sudo mv /tmp/nixos-config/* /mnt/etc/nixos
```
```
sudo mv /mnt/etc/nixos/hardware-configuration /mnt/etc/nixos/nixos
```

Install NixOS:

```
sudo nixos-install -v --show-trace --no-root-passwd --root /mnt --flake /mnt/etc/nixos#nixos
```

Home-manager:

```
home-manager switch --flake .#dingus@nixos
```

- Install home-manager using `nix shell nixpkgs#home-manager`.

# Kanata

Once the Karabine Driver is installed, run:
```
/Applications/.Karabiner-VirtualHIDDevice-Manager.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Manager activate
```

Then go to Settings > General > Driver Extensions and enable the VirtualHIDDevice.
