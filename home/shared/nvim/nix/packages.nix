# Package definitions — which categories to enable for each build variant.
#
# The key name ("nvim") becomes the binary name.
# Each package picks which categories from categories.nix are turned on.
#
# Setting `general = true` enables ALL subcategories (general.always,
# general.extra, general.snacks, general.blink).
# You can also enable specific subcategories: `general.always = true`.
let
  # Shared by every variant so they never drift.
  categories = {
    general = true; # All general.* subcategories
    treesitter = true; # Syntax highlighting
    lsp = true; # LSP servers + related plugins
    themer = true; # Colorscheme
    colorscheme = "catppuccin";
  };
in
{
  # Baked variant: lua config is frozen into the nix store. This is the
  # portable one — `nix run <flake>#nvim` works on any machine, config and
  # all. Editing lua requires a rebuild, so use `nvim-live` for iteration.
  nvim =
    { pkgs, ... }:
    {
      settings = {
        wrapRc = true; # Bake lua config into the nix store (reproducible)
        configDirName = "nvim"; # NVIM_APPNAME — used for ~/.config/<name> when wrapRc = false
      };
      inherit categories;
    };

  # Live variant: lua config is read live from ~/.config/nvim (wrapRc = false),
  # so keybind/config edits need no rebuild — just relaunch nvim. Plugins and
  # LSPs are still nix-managed (only categories.nix changes need a rebuild).
  # `aliases` gives it a `nvim` binary so tmux/shell aliases keep working.
  nvim-live =
    { pkgs, ... }:
    {
      settings = {
        wrapRc = false; # Read lua live from ~/.config/nvim instead of the store
        configDirName = "nvim";
        aliases = [ "nvim" ]; # binary is `nvim-live`; expose `nvim` too
      };
      inherit categories;
    };
}
