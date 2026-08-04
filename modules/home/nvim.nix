# Neovim (nixCats). Brings the nvim subflake's home module; per-user config
# (package variant, aliases) is set in the user modules via programs.nvim.
{ inputs, ... }:
{
  flake.modules.homeManager.nvim = {
    imports = [ inputs.nvim.homeModule ];
  };
}
