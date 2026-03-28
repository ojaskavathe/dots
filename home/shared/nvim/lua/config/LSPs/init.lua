-- LSP configurations using lzextras.lsp handler
-- Each LSP is defined as an lze spec with an 'lsp' field

require("lze").load({
	{
		"nvim-lspconfig",
		for_cat = "general.always",
		on_require = { "lspconfig" },
		-- This function is called for all specs with lsp = {...}
		lsp = function(plugin)
			vim.lsp.config(plugin.name, plugin.lsp or {})
			vim.lsp.enable(plugin.name)
		end,
		before = function(_)
			-- Set global on_attach handler
			vim.lsp.config("*", {
				on_attach = require("config.LSPs.on_attach"),
			})
		end,
	},

	-- ==========================================================================
	-- NIX
	-- ==========================================================================

	{
		"nil_ls",
		lsp = {
			filetypes = { "nix" },
		},
	},

	-- ==========================================================================
	-- LUA
	-- ==========================================================================

	{
		"lua_ls",
		lsp = {
			filetypes = { "lua" },
			settings = {
				Lua = {
					runtime = { version = "LuaJIT" },
					diagnostics = {
						globals = { "nixCats", "vim" },
					},
					telemetry = { enabled = false },
				},
			},
		},
	},

	{
		"lazydev.nvim",
		for_cat = "lsp",
		ft = "lua",
		after = function(_)
			require("lazydev").setup({})
		end,
	},

	-- ==========================================================================
	-- JAVASCRIPT/TYPESCRIPT
	-- ==========================================================================

	{
		"ts_ls",
		lsp = {
			filetypes = { "javascript", "javascriptreact", "typescript", "typescriptreact" },
		},
	},

	-- ==========================================================================
	-- WEB (CSS, HTML, TAILWIND)
	-- ==========================================================================

	{
		"css_ls",
		lsp = {
			filetypes = { "css", "scss", "less" },
			settings = {
				css = {
					validate = true,
					lint = {
						unknownAtRules = "ignore",
					},
				},
			},
		},
	},

	{
		"html",
		lsp = {
			filetypes = { "html", "htmldjango" },
		},
	},

	{
		"tailwindcss",
		lsp = {
			filetypes = { "html", "css", "javascript", "javascriptreact", "typescript", "typescriptreact" },
		},
	},

	-- ==========================================================================
	-- C/C++/CMAKE
	-- ==========================================================================

	{
		"clangd",
		lsp = {
			filetypes = { "c", "cpp", "objc", "objcpp" },
			cmd = {
				"clangd",
				"--offset-encoding=utf-16",
			},
		},
	},

	{
		"cmake",
		lsp = {
			filetypes = { "cmake" },
		},
	},

	-- ==========================================================================
	-- PYTHON
	-- ==========================================================================

	{
		"pyright",
		lsp = {
			filetypes = { "python" },
			settings = {
				pyright = {
					disableOrganizeImports = true, -- using Ruff
				},
				python = {
					analysis = {
						ignore = { "*" }, -- using Ruff
						typeCheckingMode = "off", -- using mypy
					},
				},
			},
		},
	},

	{
		"ruff",
		lsp = {
			filetypes = { "python" },
		},
	},

	-- ==========================================================================
	-- ELIXIR
	-- ==========================================================================

	{
		"elixirls",
		lsp = {
			filetypes = { "elixir", "eelixir" },
			cmd = { "elixir-ls" },
		},
	},

	-- ==========================================================================
	-- VALA
	-- ==========================================================================

	{
		"vala_ls",
		lsp = {
			filetypes = { "vala" },
		},
	},

	-- ==========================================================================
	-- RUST (handled by rustaceanvim)
	-- ==========================================================================

	{
		"crates.nvim",
		for_cat = "lsp",
		ft = "toml",
		after = function(_)
			require("crates").setup()
		end,
	},

	-- ==========================================================================
	-- FORMATTING (conform.nvim)
	-- ==========================================================================

	{
		"conform.nvim",
		for_cat = "lsp",
		event = { "BufReadPre", "BufNewFile" },
		after = function(_)
			require("conform").setup({
				formatters_by_ft = {
					lua = { "stylua" },
					nix = { "nixfmt" },
					python = { "ruff_format" },
					javascript = { "prettier" },
					javascriptreact = { "prettier" },
					typescript = { "prettier" },
					typescriptreact = { "prettier" },
					html = { "prettier" },
					css = { "prettier" },
					json = { "prettier" },
					yaml = { "prettier" },
					markdown = { "prettier" },
					c = { "clang-format" },
					cpp = { "clang-format" },
				},
			})
		end,
	},
})
