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
      ${builtins.readFile ./diagnostics.lua}
    '';

    plugins = with pkgs.vimPlugins; [
      nvim-web-devicons
      (nvim-treesitter.withPlugins (
        plugins: with plugins; [
          nix
          vim
          bash
          lua
          json
          python
        ]
      ))
      {
        plugin = nvim-treesitter-textsubjects;
        type = "lua";
        config = ''
          require('nvim-treesitter.configs').setup {
            textsubjects = {
              enable = true,
              prev_selection = ',', -- (Optional) keymap to select the previous selection
              keymaps = {
                ['.'] = 'textsubjects-smart',
                [';'] = 'textsubjects-container-outer',
                ['i;'] = { 'textsubjects-container-inner', desc = "Select inside containers (classes, functions, etc.)" },
              },
            },
          }
        '';
      }
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
      telescope-fzf-native-nvim
      telescope-ui-select-nvim
      telescope-symbols-nvim
      {
        plugin = telescope-nvim;
        type = "lua";
        config = builtins.readFile ./plugins/telescope.lua;
      }
    ];
  };
}
