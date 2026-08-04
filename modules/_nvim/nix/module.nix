# Wrapper-module config for neovim (nix-wrapper-modules).
#
# Replaces the old nixCats categories.nix + packages.nix. There are no
# categories/packages here — everything is a `config.specs.<name>`:
#   - lazy = false  -> loaded at startup            (was startupPlugins)
#   - lazy = true   -> pack/opt, loaded on demand   (was optionalPlugins)
#   - runtimePkgs   -> binaries added to nvim's PATH (was lspsAndRuntimeDeps)
#
# The lua config is baked in (config_directory points at the flake source), so
# editing lua needs a rebuild. lze/lzextras come from the `plugins-*` inputs.
# The lua `for_cat` handler / `nixCats(...)` calls read settings through the
# info plugin — see the shim at the top of lua/config/init.lua.
inputs:
{
  config,
  wlib,
  lib,
  pkgs,
  options,
  ...
}:
{
  imports = [ wlib.wrapperModules.neovim ];

  # Plugins auto-built from `plugins-*` flake inputs, exposed as
  # config.nvim-lib.neovimPlugins.<name-without-prefix> (e.g. .lze, .lzextras).
  options.nvim-lib.neovimPlugins = lib.mkOption {
    readOnly = true;
    type = lib.types.attrsOf wlib.types.stringable;
    default = config.nvim-lib.pluginsFromPrefix "plugins-" inputs;
  };
  options.nvim-lib.pluginsFromPrefix = lib.mkOption {
    type = lib.types.raw;
    readOnly = true;
    default =
      prefix: inputs:
      lib.pipe inputs [
        builtins.attrNames
        (builtins.filter (s: lib.hasPrefix prefix s))
        (map (
          input:
          let
            name = lib.removePrefix prefix input;
          in
          {
            inherit name;
            value = config.nvim-lib.mkPlugin name inputs.${input};
          }
        ))
        builtins.listToAttrs
      ];
  };

  # Bake the lua config. ../. is the _nvim flake root (holds init.lua + lua/).
  config.settings.config_directory = ../.;

  # Colorscheme name, read in lua via nixCats("colorscheme").
  options.settings.colorscheme = lib.mkOption {
    type = lib.types.str;
    default = "catppuccin";
  };

  # --- startup: loaded immediately, before init.lua ---
  config.specs.startup = {
    lazy = false;
    data = [
      config.nvim-lib.neovimPlugins.lze # lazy-loading engine (must be at startup)
      config.nvim-lib.neovimPlugins.lzextras # extra lze handlers (lsp handler)
    ]
    ++ (with pkgs.vimPlugins; [
      plenary-nvim # lua utility library (dependency of many plugins)
      vim-tmux-navigator # must be startup so tmux's is_vim check works
      vim-obsession # session tracking (:Obsession)
      vim-sleuth # auto-detect indentation from file contents
      nvim-web-devicons # nerd font icons (lualine, snacks, oil, ...)
      catppuccin-nvim # colorscheme (loaded early so the UI isn't unstyled)
      nvim-treesitter.withAllGrammars
    ]);
  };

  # --- lazy: pack/opt, loaded on demand by lze (see lua/config/plugins/*) ---
  config.specs.plugins = {
    lazy = true;
    data = with pkgs.vimPlugins; [
      # always-on
      nvim-lspconfig # LSP client configuration
      lualine-nvim # statusline
      gitsigns-nvim # git signs + hunk operations
      vim-fugitive # git commands (:Git, :G, :Gdiffsplit)
      nvim-surround # surround text objects (cs, ds, ys)
      # extra
      supermaven-nvim # AI inline completions
      oil-nvim # file explorer (-)
      grug-far-nvim # find & replace
      comment-nvim # toggle comments (gc, gb)
      nvim-autopairs # auto-close brackets
      nvim-ts-context-commentstring # treesitter-aware comment styles
      indent-blankline-nvim # indent guides
      trouble-nvim # diagnostics list
      outline-nvim # code outline sidebar
      which-key-nvim # keymap hints
      render-markdown-nvim # inline markdown rendering
      neo-tree-nvim # file tree sidebar
      nui-nvim # UI library (neo-tree dependency)
      colorful-winsep-nvim # highlight active window separator
      # picker
      snacks-nvim # picker + gitbrowse (replaces telescope)
      # completion
      blink-cmp # completion engine
      blink-compat # nvim-cmp source compatibility layer
      luasnip # snippet engine
      cmp-cmdline # cmdline completion source (via blink.compat)
      # lsp-adjacent
      conform-nvim # formatter (replaces none-ls)
      undotree # undo history visualizer
      lazydev-nvim # neovim API docs for lua development
      rustaceanvim # rust tools (LSP + debugging)
      crates-nvim # Cargo.toml dependency helper
    ];
  };

  # --- runtime deps: LSPs, formatters, tools on nvim's PATH ---
  config.specs.tools = {
    data = null;
    runtimePkgs = with pkgs; [
      nil # Nix LSP
      nixfmt # Nix formatter (conform)
      lua-language-server # Lua LSP
      stylua # Lua formatter (conform)
      vala-language-server
      typescript-language-server
      prettier # JS/TS/HTML/CSS formatter (conform)
      vscode-langservers-extracted # HTML, CSS, JSON, ESLint LSPs
      tailwindcss-language-server
      clang-tools # clangd + clang-format (conform)
      cmake-language-server
      pyright # Python LSP
      ruff # Python linter/formatter (LSP + conform)
      dexter # Elixir LSP (remoteoss/dexter)
      ripgrep # snacks picker + grug-far
      fd # snacks picker file finding
    ];
  };

  # Collect each enabled spec's runtimePkgs onto the wrapper PATH.
  # (tips-and-tricks pattern from the neovim wrapper docs.)
  config.specMods =
    {
      parentSpec ? null,
      parentOpts ? null,
      parentName ? null,
      config,
      ...
    }:
    {
      options.runtimePkgs = options.runtimePkgs // {
        description = ''
          Packages to add to the neovim wrapper PATH when this spec is enabled.
        '';
      };
    };
  config.runtimePkgs = config.specCollect (acc: v: acc ++ (v.runtimePkgs or [ ])) [ ];
}
