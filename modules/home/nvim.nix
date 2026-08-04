# Neovim (nix-wrapper-modules). Brings the nvim subflake's home module;
# per-user config (aliases, enable) is set in the user modules via programs.nvim.
{ inputs, ... }:
{
  flake.modules.homeManager.nvim = {
    imports = [ inputs.nvim.homeModule ];
  };

  # Re-export the subflake's package at the top level so it's reachable as
  # `nix run github:ojaskavathe/dots#nvim` (no ?dir= needed).
  perSystem =
    { system, ... }:
    {
      packages.nvim = inputs.nvim.packages.${system}.default;
    };
}
