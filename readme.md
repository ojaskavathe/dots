Clone the repository in /tmp, then partition and format using disko:

```
sudo nix --experimental-features "nix-command flakes" run github:nix-community/disko -- --mode disko /tmp/nixos-config/disk-config.nix
```

Setup config:

```
sudo nixos-generate-config --no-filesystems --root /mnt
```
```
sudo mv /tmp/nixos-config/*.nix /mnt/etc/nixos
```

Install NixOS:

```
sudo nixos-install -v --show-trace --no-root-passwd --root /mnt --flake /mnt/etc/nixos#nixos
```