# Category definitions — what plugins/tools are available in each category.
#
# nixCats has a few top-level "slot" names with special meaning:
#   - startupPlugins:     Loaded immediately (pack/*/start/)
#   - optionalPlugins:    Available but not loaded until triggered (pack/*/opt/)
#   - lspsAndRuntimeDeps: Added to PATH so nvim and plugins can find them
#   - sharedLibraries:    Added to LD_LIBRARY_PATH
#   - environmentVariables: Set in the nvim wrapper
#   - extraLuaPackages:   Added to LUA_PATH/LUA_CPATH
#   - extraPython3Packages: Available to python3 host
#
# Within each slot, you define your own category names (general, lsp, etc.).
# These names match what you enable in packages.nix and check with
# nixCats("category.name") in lua.
#
# Subcategories (general.always, general.extra) let you group plugins.
# Enabling `general = true` turns on ALL subcategories.
# Enabling `general.always = true` turns on only that subcategory.

{
  pkgs,
  settings,
  categories,
  extra,
  name,
  ...
}@packageDef:
{
  # Startup plugins — loaded immediately, before init.lua runs.
  # Keep this minimal: only things that MUST be available at startup.
  startupPlugins = {
    general = {
      always = with pkgs; [
        neovimPlugins.lze # Lazy-loading engine (must be startup so lze.load works)
        neovimPlugins.lzextras # Extra lze handlers (lsp handler)
        vimPlugins.plenary-nvim # Lua utility library (dependency of many plugins)
        vimPlugins.vim-tmux-navigator # Must be startup so tmux's is_vim check works
        vimPlugins.vim-obsession # Session tracking (:Obsession)
        vimPlugins.vim-sleuth # Auto-detect indentation from file contents
      ];
      extra = with pkgs.vimPlugins; [
        nvim-web-devicons # Nerd font icons (used by lualine, telescope, oil, etc.)
      ];
    };

    # Colorscheme — must load at startup so the UI isn't briefly unstyled.
    # The `categories.colorscheme` value (set in packages.nix) picks which one.
    themer =
      with pkgs.vimPlugins;
      (builtins.getAttr (categories.colorscheme or "catppuccin") {
        "catppuccin" = catppuccin-nvim;
        "catppuccin-mocha" = catppuccin-nvim;
        "onedark" = onedark-nvim;
      });

    treesitter = with pkgs.vimPlugins; [
      (nvim-treesitter.withAllGrammars)
    ];
  };

  # Optional plugins — placed in pack/*/opt/, loaded on demand by lze.
  # Each plugin needs a corresponding lze spec in lua/ to define its trigger
  # (event, cmd, keys, ft, etc.) — otherwise it never loads.
  optionalPlugins = {
    general = {
      always = with pkgs.vimPlugins; [
        nvim-lspconfig # LSP client configuration
        lualine-nvim # Statusline
        gitsigns-nvim # Git signs in gutter + hunk operations
        vim-fugitive # Git commands (:Git, :G, :Gdiffsplit)
        nvim-surround # Surround text objects (cs, ds, ys)
      ];

      extra = with pkgs.vimPlugins; [
        supermaven-nvim # AI inline completions
        oil-nvim # File explorer (-)
        grug-far-nvim # Find & replace
        comment-nvim # Toggle comments (gc, gb)
        nvim-autopairs # Auto-close brackets
        nvim-ts-context-commentstring # Treesitter-aware comment styles
        indent-blankline-nvim # Indent guides
        trouble-nvim # Diagnostics list
        outline-nvim # Code outline sidebar
        which-key-nvim # Keymap hints
        render-markdown-nvim # Inline markdown rendering
        neo-tree-nvim # File tree sidebar
        nui-nvim # UI library (neo-tree dependency)
        colorful-winsep-nvim # Highlight active window separator
      ];

      telescope = with pkgs.vimPlugins; [
        telescope-nvim
        telescope-fzf-native-nvim # Fast fuzzy matching
        telescope-ui-select-nvim # Use telescope for vim.ui.select
        telescope-symbols-nvim # Symbol picker
      ];

      blink = with pkgs.vimPlugins; [
        blink-cmp # Completion engine
        blink-compat # nvim-cmp source compatibility layer
        luasnip # Snippet engine
        cmp-cmdline # Cmdline completion source (via blink.compat)
      ];
    };

    lsp = with pkgs.vimPlugins; [
      conform-nvim # Formatter (replaces none-ls)
      undotree # Undo history visualizer
      lazydev-nvim # Neovim API docs for lua development
      rustaceanvim # Rust tools (LSP + debugging)
      crates-nvim # Cargo.toml dependency helper
    ];
  };

  # LSP servers and tools — added to PATH inside the nvim wrapper.
  # These are NOT vim plugins, they're standalone binaries that nvim/plugins invoke.
  lspsAndRuntimeDeps = {
    general = with pkgs; [
      nil # Nix LSP
      nixfmt # Nix formatter (used by conform)
      lua-language-server # Lua LSP
      stylua # Lua formatter (used by conform)
      vala-language-server
      nodePackages.typescript-language-server
      nodePackages.prettier # JS/TS/HTML/CSS formatter (used by conform)
      vscode-langservers-extracted # HTML, CSS, JSON, ESLint LSPs
      tailwindcss-language-server
      clang-tools # clangd + clang-format (used by conform)
      cmake-language-server
      pyright # Python LSP
      ruff # Python linter/formatter (used as LSP + conform)
      elixir-ls
      ripgrep # Used by telescope and grug-far
      fd # Used by telescope for file finding
    ];
  };

  # Unused but required by nixCats schema
  sharedLibraries = {
    general = with pkgs; [ ];
  };
  environmentVariables = {
    general = { };
  };
  extraLuaPackages = {
    general = [ (_: [ ]) ];
  };
  extraPython3Packages = {
    general = [ (_: [ ]) ];
  };
}
