{ pkgs, ... }: {

  home.file.".config/nvim/lua/config/statuscol.lua".source = ./statuscol.lua;

  programs.neovim = {
    enable = true;
    defaultEditor = true;

    vimAlias = true;
    viAlias = true;
    vimdiffAlias = true;


    extraLuaConfig = ''
      ${builtins.readFile ./options.lua}
      ${builtins.readFile ./keymap.lua}
      vim.o.statuscolumn = "%!v:lua.require'config.statuscol'.statuscolumn()"
    '';

    plugins = with pkgs.vimPlugins; [
      {
        plugin = comment-nvim;
        type = "lua";
        config = ''require("Comment").setup()'';
      }
      {
        plugin = gitsigns-nvim;
        type = "lua";
        config = builtins.readFile ./plugins/gitsigns.lua;
      }
      vim-fugitive
    ];
  };
}
