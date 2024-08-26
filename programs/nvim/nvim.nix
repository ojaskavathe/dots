{ pkgs, ... }:
{

  xdg.configFile = {
    "nvim/lua/statuscol.lua".source = ./statuscol.lua;
  };

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
      {
        plugin = catppuccin-nvim;
        type = "lua";
        config = ''
          require("catppuccin").setup {
            flavour = "mocha",
            integrations = {
              telescope = {
                enabled = true,
                style = "nvchad"
              },
            }
          }
          vim.cmd.colorscheme "catppuccin"
        '';
      }
      nvim-web-devicons
      {
        plugin = (
          nvim-treesitter.withPlugins (
            plugins: with plugins; [
              nix
              vim
              bash
              lua
              json
              python
            ]
          )
        );
        type = "lua";
        config = builtins.readFile ./plugins/treesitter.lua;
      }
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
      vim-sleuth
      vim-fugitive
      vim-obsession
      telescope-fzf-native-nvim
      telescope-ui-select-nvim
      telescope-symbols-nvim
      {
        plugin = telescope-nvim;
        type = "lua";
        config = builtins.readFile ./plugins/telescope.lua;
      }
      {
        plugin = indent-blankline-nvim;
        type = "lua";
        config = ''require("ibl").setup {}'';
      }
      {
        plugin = lualine-nvim;
        type = "lua";
        config = builtins.readFile ./plugins/lualine.lua;
      }
      { plugin = vim-tmux-navigator; }
      {
        plugin = nvim-surround;
        type = "lua";
        config = ''require("nvim-surround").setup {}'';
      }
      {
        plugin = nvim-autopairs;
        type = "lua";
        config = ''require("nvim-autopairs").setup {}'';
      }
      {
        plugin = trouble-nvim;
        type = "lua";
        config = ''
          require("trouble").setup(opts)

          --keybinds
          vim.keymap.set("n", "<leader>xx", function() require("trouble").toggle() end)
          vim.keymap.set("n", "<leader>xw", function() require("trouble").toggle("workspace_diagnostics") end)
          vim.keymap.set("n", "<leader>xd", function() require("trouble").toggle("document_diagnostics") end)
          vim.keymap.set("n", "<leader>xq", function() require("trouble").toggle("quickfix") end)
          vim.keymap.set("n", "<leader>xl", function() require("trouble").toggle("loclist") end)
          vim.keymap.set("n", "gR", function() require("trouble").toggle("lsp_references") end)
        '';
      }

      # nav
      {
        plugin = oil-nvim;
        type = "lua";
        config = ''
          require("oil").setup()
          vim.keymap.set("n", "-", "<CMD>Oil<CR>", { desc = "Open parent directory" })
        '';
      }

      # lsp stuff
      {
        plugin = nvim-lspconfig;
        type = "lua";
        config = builtins.readFile ./plugins/lspconfig.lua;
      }
      {
        plugin = none-ls-nvim;
        type = "lua";
        config = builtins.readFile ./plugins/none-ls.lua;
      }

      # cmp
      cmp-nvim-lsp
      cmp-buffer
      cmp-path
      cmp-cmdline
      cmp-nvim-lsp-signature-help
      cmp_luasnip
      luasnip
      {
        plugin = lazydev-nvim;
        type = "lua";
        config = ''require("lazydev.config").setup {}'';
      }
      {
        plugin = nvim-cmp;
        type = "lua";
        config = builtins.readFile ./plugins/nvim-cmp.lua;
      }
    ];

    extraPackages = with pkgs; [
      # lsp
      nil
      lua-language-server

      # none-ls
      nixfmt-rfc-style
      stylua
    ];
  };
}
