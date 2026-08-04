# dots

my nix config for macos (nix-darwin) and nixos, plus home-manager. built with
[flake-parts] + [import-tree] in the [dendritic] style: every file under
`modules/` is a flake-parts module, and each reusable piece is exposed as
`flake.modules.<class>.<name>` — so a downstream flake can pull them in and
build on top (that's what my private `dotdot` does).

## hosts

- `camille` — macbook pro (aarch64-darwin)
- `tuf` — asus tuf a15 (nixos)
- `galio-wsl` — nixos on wsl

## layout

```
modules/
  home/     home-manager feature modules (git, zsh, tmux, stylix, ...)
  users/    per-user home configs (ojas, dingus)
  darwin/   nix-darwin modules + base bundle
  nixos/    nixos modules + base bundle
  hosts/    host assemblies (camille, tuf, galio-wsl)
  _nvim/    neovim (nix-wrapper-modules), a vendored subflake
  flake-modules.nix   nixpkgs.nix   # shared bits
```

paths with a `_` component are ignored by import-tree — subflakes (`_nvim`),
host hardware (`hosts/_tuf`), and non-module helpers (`home/_hyprland`).

## use

```
nrs   # system rebuild — darwin or nixos, picked by host
hms   # home-manager switch --flake .#<user>@<host>
```

## neovim

my neovim config is a self-contained package — run it anywhere, no clone:

```
nix run github:ojaskavathe/dots#nvim
```

config is baked into the build (it won't read your `~/.config/nvim`). that's
what makes it portable.

## reinstall (nixos)

```
sudo nix run github:nix-community/disko -- --mode disko \
  --argstr target /dev/nvme0n1 ./modules/hosts/_tuf/disko-configuration.nix
sudo nixos-install --flake .#tuf
```

## kanata on macos

install [Karabiner-DriverKit-VirtualHIDDevice] and activate it:

```
/Applications/.Karabiner-VirtualHIDDevice-Manager.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Manager activate
```

enable it under Settings > General > Login Items & Extensions > Driver
Extensions. a launchd daemon then runs kanata on boot (logs in `/tmp`). add
kanata, tmux, and kitty to Settings > Privacy & Security > Input Monitoring
(from their `~/.nix-profile/bin` symlinks).

## windows (kanata only)

from an elevated powershell:

```
irm https://raw.githubusercontent.com/ojaskavathe/dots/master/windows/setup.ps1 | iex
```

installs kanata, fetches the latest keyboard config, runs it at login. re-run to
update.

[flake-parts]: https://flake.parts
[import-tree]: https://github.com/vic/import-tree
[dendritic]: https://github.com/mightyiam/dendritic
[Karabiner-DriverKit-VirtualHIDDevice]: https://github.com/pqrs-org/Karabiner-DriverKit-VirtualHIDDevice/tree/main/dist
