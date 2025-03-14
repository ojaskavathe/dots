{
  pkgs,
  pkgs-stable,
  lib,
  config,
  ...
}:
{

  options = {
    nvim = {
      enable = lib.mkEnableOption "Enable Neovim";
    };
  };

  config = lib.mkIf config.nvim.enable {
    xdg.configFile = {
      "nvim/lua/statuscol.lua".source = ./statuscol.lua;
    };

    programs.neovim = {
      enable = true;
      defaultEditor = true;

      vimAlias = true;
      viAlias = true;
      vimdiffAlias = true;

      extraLuaConfig = 
        # lua
        ''
        -- until https://github.com/yetone/avante.nvim/pull/1345 is merged
        package.cpath = package.cpath .. ";${pkgs.vimPlugins.avante-nvim}/build/?.${
          if pkgs.stdenv.isDarwin then "dylib" else "so"
        }"

        ${builtins.readFile ./options.lua}
        ${builtins.readFile ./keymap.lua}
        ${builtins.readFile ./diagnostics.lua}
      '';

      plugins = with pkgs.vimPlugins; [
        {
          plugin = catppuccin-nvim;
          type = "lua";
          config =
            # lua
            ''
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
          plugin = nvim-treesitter.withAllGrammars;
          type = "lua";
          config = builtins.readFile ./plugins/treesitter.lua;
        }
        {
          plugin = nvim-treesitter-textsubjects;
          type = "lua";
          config = # lua
            ''
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
          plugin = nvim-ts-context-commentstring;
          type = "lua";
          config = # lua
            ''
              require('ts_context_commentstring').setup {
                enable_autocmd = false,
              }
            '';
        }
        {
          plugin = comment-nvim;
          type = "lua";
          config = # lua
            ''
              require("Comment").setup {
                pre_hook = require('ts_context_commentstring.integrations.comment_nvim').create_pre_hook(),
              }
            '';
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
          config = # lua
            ''require("ibl").setup {}'';
        }
        {
          plugin = lualine-nvim;
          type = "lua";
          config = builtins.readFile ./plugins/lualine.lua;
        }
        vim-tmux-navigator
        {
          plugin = nvim-surround;
          type = "lua";
          config = # lua
            ''require("nvim-surround").setup {}'';
        }
        {
          plugin = nvim-autopairs;
          type = "lua";
          config = ''require("nvim-autopairs").setup {}'';
        }
        {
          plugin = trouble-nvim;
          type = "lua";
          config =
            # lua
            ''
              require("trouble").setup(opts)

              --keybinds
              vim.keymap.set("n", "<leader>xx", function() require("trouble").toggle("diagnostics") end)
              vim.keymap.set("n", "<leader>xq", function() require("trouble").toggle("quickfix") end)
              vim.keymap.set("n", "<leader>xl", function() require("trouble").toggle("loclist") end)
              vim.keymap.set("n", "gR", function() require("trouble").toggle("lsp_references") end)
            '';
        }
        {
          plugin = outline-nvim;
          type = "lua";
          config =
            # lua
            ''
              require("outline").setup({})
              vim.keymap.set("n", "<leader>o", "<cmd>Outline<CR>")
            '';
        }

        # nav
        {
          plugin = oil-nvim;
          type = "lua";
          config = builtins.readFile ./plugins/oil.lua;
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
        {
          # neotest 5.8.0 currently broken
          plugin = pkgs-stable.vimPlugins.rustaceanvim;
          type = "lua";
        }
        {
          plugin = crates-nvim;
          type = "lua";
          config = ''
            require("crates").setup()
          '';
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

        {
          plugin = avante-nvim;
          type = "lua";
          config = builtins.readFile ./plugins/avante.lua;
        }
      ];

      extraPackages = with pkgs; [
        # nix
        nil
        nixfmt-rfc-style

        # lua
        lua-language-server
        stylua

        # vala
        vala-language-server

        # js/ts
        nodePackages.typescript-language-server
        nodePackages.prettier
        vscode-langservers-extracted

        tailwindcss-language-server

        # cpp
        clang-tools
        cmake-language-server
      ];
    };
  };
}
