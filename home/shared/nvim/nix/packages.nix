# Package definitions — which categories to enable for each build variant.
#
# The key name ("nvim") becomes the binary name.
# Each package picks which categories from categories.nix are turned on.
#
# Setting `general = true` enables ALL subcategories (general.always,
# general.extra, general.telescope, general.blink).
# You can also enable specific subcategories: `general.always = true`.
{
  nvim =
    { pkgs, ... }:
    {
      settings = {
        wrapRc = true; # Bake lua config into the nix store (reproducible)
        configDirName = "nvim"; # NVIM_APPNAME — used for ~/.config/<name> when wrapRc = false
      };

      categories = {
        general = true; # All general.* subcategories
        treesitter = true; # Syntax highlighting
        lsp = true; # LSP servers + related plugins
        themer = true; # Colorscheme
        colorscheme = "catppuccin";
      };
    };
}
